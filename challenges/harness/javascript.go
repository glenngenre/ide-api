package harness

import "fmt"

type JavaScriptBuilder struct{}

func (b *JavaScriptBuilder) Build(functionName string, userCode string) (string, error) {
	return fmt.Sprintf(`%s

const __json = require("fs").readFileSync(0, "utf8");
let __line;
try {
  const __args = JSON.parse(__json);
  if (!Array.isArray(__args)) throw new Error("stdin must be a JSON array of arguments");
  const __result = (%s)(...__args);
  __line = { status: "ok", value: __result === undefined ? null : __result };
} catch (__error) {
  __line = { status: "runtime_error", error: __error.name || "Error",
    message: String(__error.message || __error), trace: String(__error.stack || __error) };
}
process.stdout.write("\x1e__SKWTR__ " + JSON.stringify(__line) + "\n");
`, userCode, functionName), nil
}
