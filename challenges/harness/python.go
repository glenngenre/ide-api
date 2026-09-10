package harness

import "fmt"

type PythonBuilder struct{}

func (b *PythonBuilder) Build(functionName string, userCode string, casesJSON string) (string, error) {
	harness := fmt.Sprintf(`%s

import json as __json, sys as __sys, io as __io, traceback as __tb, contextlib as __ctx
__CASES = %s
__OUT = __sys.stdout
for __i, __args in enumerate(__CASES):
    __buf = __io.StringIO()
    try:
        with __ctx.redirect_stdout(__buf):
            __r = %s(*__args)
        __line = {"i": __i, "ok": True, "out": __r, "stdout": __buf.getvalue()}
        __text = __json.dumps(__line)
    except BaseException:
        __line = {"i": __i, "ok": False, "err": __tb.format_exc(), "stdout": __buf.getvalue()}
        __text = __json.dumps(__line)
    __OUT.write("__SKWTR__ " + __text + "\n")
`, userCode, casesJSON, functionName)

	return harness, nil
}
