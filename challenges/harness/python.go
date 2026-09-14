package harness

import "fmt"

type PythonBuilder struct{}

func (b *PythonBuilder) Build(functionName string, userCode string) (string, error) {
	return fmt.Sprintf(`%s

import json as __json, sys as __sys, traceback as __tb
try:
    __args = __json.loads(__sys.stdin.read())
    if not isinstance(__args, list):
        raise ValueError("stdin must be a JSON array of arguments")
    __fn = globals().get(%q)
    if not callable(__fn):
        __line = {"status": "bad_signature", "message": "No callable named " + %q}
    else:
        __result = __fn(*__args)
        __line = {"status": "ok", "value": __result}
except BaseException as __error:
    __line = {"status": "runtime_error", "error": type(__error).__name__,
              "message": str(__error), "trace": __tb.format_exc()}
__sys.stdout.write("\x1e__SKWTR__ " + __json.dumps(__line) + "\n")
`, userCode, functionName, functionName), nil
}
