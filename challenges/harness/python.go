package harness

import "fmt"

type PythonBuilder struct{}

func (b *PythonBuilder) WrapWithStdin(functionName, userCode string) string {
	return fmt.Sprintf(`import json as __skwtr_json, sys as __skwtr_sys
__skwtr_write = __skwtr_sys.stdout.write
__skwtr_flush = __skwtr_sys.stdout.flush

%s

__skwtr_args = __skwtr_json.loads(__skwtr_sys.stdin.read())
__skwtr_result = %s(*__skwtr_args)
__skwtr_write(%q + __skwtr_json.dumps(__skwtr_result, allow_nan=False) + "\n")
__skwtr_flush()
`, userCode, functionName, ResultPrefix)
}
