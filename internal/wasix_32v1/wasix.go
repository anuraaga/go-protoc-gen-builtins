package wasix_32v1

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

const ModuleName = "wasix_32v1"

const i32, i64 = api.ValueTypeI32, api.ValueTypeI64

// MustInstantiate calls Instantiate or panics on error.
//
// This is a simpler function for those who know the module ModuleName is not
// already instantiated, and don't need to unload it.
func MustInstantiate(ctx context.Context, r wazero.Runtime) {
	if _, err := Instantiate(ctx, r); err != nil {
		panic(err)
	}
}

// Instantiate instantiates the ModuleName module into the runtime.
//
// # Notes
//
//   - Failure cases are documented on wazero.Runtime InstantiateModule.
//   - Closing the wazero.Runtime has the same effect as closing the result.
func Instantiate(ctx context.Context, r wazero.Runtime) (api.Closer, error) {
	return NewBuilder(r).Instantiate(ctx)
}

// Builder configures the ModuleName module for later use via Compile or Instantiate.
//
// # Notes
//
//   - This is an interface for decoupling, not third-party implementations.
//     All implementations are in wazero.
type Builder interface {
	// Compile compiles the ModuleName module. Call this before Instantiate.
	//
	// Note: This has the same effect as the same function on wazero.HostModuleBuilder.
	Compile(context.Context) (wazero.CompiledModule, error)

	// Instantiate instantiates the ModuleName module and returns a function to close it.
	//
	// Note: This has the same effect as the same function on wazero.HostModuleBuilder.
	Instantiate(context.Context) (api.Closer, error)
}

// NewBuilder returns a new Builder.
func NewBuilder(r wazero.Runtime) Builder {
	return &builder{r}
}

type builder struct{ r wazero.Runtime }

// hostModuleBuilder returns a new wazero.HostModuleBuilder for ModuleName
func (b *builder) hostModuleBuilder() wazero.HostModuleBuilder {
	ret := b.r.NewHostModuleBuilder(ModuleName)
	exportFunctions(ret)
	return ret
}

// Compile implements Builder.Compile
func (b *builder) Compile(ctx context.Context) (wazero.CompiledModule, error) {
	return b.hostModuleBuilder().Compile(ctx)
}

// Instantiate implements Builder.Instantiate
func (b *builder) Instantiate(ctx context.Context) (api.Closer, error) {
	return b.hostModuleBuilder().Instantiate(ctx)
}

func exportFunctions(builder wazero.HostModuleBuilder) {
	builder.NewFunctionBuilder().
		WithGoModuleFunction(callbackSignalFn, []api.ValueType{i32, i32}, []api.ValueType{}).
		Export("callback_signal")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(fdDupFn, []api.ValueType{i32, i32}, []api.ValueType{i32}).
		Export("fd_dup")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(futexWaitFn, []api.ValueType{i32, i32, i32, i32}, []api.ValueType{i32}).
		Export("futex_wait")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(futexWakeFn, []api.ValueType{i32, i32}, []api.ValueType{i32}).
		Export("futex_wake")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(futexWakeAllFn, []api.ValueType{i32, i32}, []api.ValueType{i32}).
		Export("futex_wake_all")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(stackCheckpointFn, []api.ValueType{i32, i32}, []api.ValueType{i32}).
		Export("stack_checkpoint")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(stackRestoreFn, []api.ValueType{i32, i64}, []api.ValueType{}).
		Export("stack_restore")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(threadExitFn, []api.ValueType{i32}, []api.ValueType{}).
		Export("thread_exit")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(threadParallelismFn, []api.ValueType{i32}, []api.ValueType{i32}).
		Export("thread_parallelism")

	builder.NewFunctionBuilder().
		WithGoModuleFunction(threadSignalFn, []api.ValueType{i32, i32}, []api.ValueType{i32}).
		Export("thread_signal")
}

var callbackSignalFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) {
	// We do not execute the wasm module concurrently so only have a single thread, we
	// can ignore signals.
	stack[0] = 0
})

var fdDupFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, _ []uint64) {
	// We do not support child plugins so never call this.
	panic("fd_dup")
})

var futexWaitFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, _ []uint64) {
	// We do not execute the wasm module concurrently so know this is never called.
	panic("futex_wait")
})

var futexWakeFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, _ []uint64) {
	// We do not execute the wasm module concurrently so know this is never called.
	panic("futex_wake")
})

var futexWakeAllFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, _ []uint64) {
	// We do not execute the wasm module concurrently so know this is never called.
	panic("futex_wake_all")
})

var stackCheckpointFn = api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
	snapshotPtr := stack[0]
	retValPtr := stack[1]
	d := ctx.Value(wasixDataKey{}).(*wasixData)

	// We go ahead and save the entire C-stack for now
	cstackPointer := uint32(mod.ExportedGlobal("__stack_pointer").Get())
	cstackTop := uint32(mod.ExportedGlobal("__heap_base").Get())
	cstackView, ok := mod.Memory().Read(cstackPointer, cstackTop-cstackPointer)
	if !ok {
		panic("read failed")
	}
	cstack := make([]byte, len(cstackView))
	copy(cstack, cstackView)

	sc := ctx.Value(experimental.SnapshotterKey{}).(experimental.Snapshotter)
	s := sc.Snapshot()

	idx := len(d.checkpoints)
	d.checkpoints = append(d.checkpoints, wasixCheckpoint{
		snapshot:      s,
		retValPtr:     uint32(retValPtr),
		cstackPointer: cstackPointer,
		cstack:        cstack,
	})

	// pub struct StackSnapshot {
	//    pub user: u64,
	//    pub hash: u128,
	// }
	if !mod.Memory().WriteUint64Le(uint32(snapshotPtr), uint64(idx)) {
		panic("write failed")
	}
	if !mod.Memory().WriteUint64Le(uint32(retValPtr), 0) {
		panic("write failed")
	}

	stack[0] = 0
})

var stackRestoreFn = api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
	snapshotPtr := stack[0]
	ret := stack[1]

	snapshotIdx, ok := mod.Memory().ReadUint64Le(uint32(snapshotPtr))
	if !ok {
		panic("read failed")
	}

	d := ctx.Value(wasixDataKey{}).(*wasixData)
	c := d.checkpoints[snapshotIdx]

	mod.ExportedGlobal("__stack_pointer").(api.MutableGlobal).Set(uint64(c.cstackPointer))
	mod.Memory().Write(c.cstackPointer, c.cstack)

	mod.Memory().WriteUint64Le(c.retValPtr, ret)
	stack[0] = 0
	c.snapshot.Restore(stack[:1])
})

var threadExitFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, _ []uint64) {
	// We do not execute the wasm module concurrently so know this is never called.
	panic("thread_exit")
})

var threadParallelismFn = api.GoModuleFunc(func(_ context.Context, m api.Module, stack []uint64) {
	// We do not execute the wasm module concurrently so force this to 1, as if 1 CPU.
	resPtr := uint32(stack[0])
	m.Memory().WriteUint32Le(resPtr, 1)
	stack[0] = 0
})

var threadSignalFn = api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) {
	// We do not execute the wasm module concurrently so only have a single thread, we
	// can ignore signals.
	stack[0] = 0
})

type wasixCheckpoint struct {
	snapshot      experimental.Snapshot
	retValPtr     uint32
	cstackPointer uint32
	cstack        []byte
}

type wasixData struct {
	checkpoints []wasixCheckpoint
}

type wasixDataKey struct{}

func BackgroundContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, experimental.EnableSnapshotterKey{}, true)
	ctx = context.WithValue(ctx, wasixDataKey{}, &wasixData{})
	return ctx
}
