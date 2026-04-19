module github.com/wasilibs/go-protoc-gen-builtins

go 1.25.0

require github.com/tetratelabs/wazero v1.11.0

require github.com/wasilibs/wazero-helpers v0.0.0-20250123031827-cd30c44769bb

require (
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/tetratelabs/wazero => github.com/evacchi/wazero v1.7.1-0.20260418141118-6a31d4ad01e1
