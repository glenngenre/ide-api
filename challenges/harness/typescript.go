package harness

import "fmt"

type TypeScriptBuilder struct{}

func (b *TypeScriptBuilder) Build(functionName string, userCode string) (string, error) {
	return fmt.Sprintf(`%s

import * as fs from "fs";
let __line: any;
try {
  const __args = JSON.parse(fs.readFileSync(0, "utf8"));
  if (!Array.isArray(__args)) throw new Error("stdin must be a JSON array of arguments");
  if (typeof (%s as any) !== "function") {
    __line = { status: "bad_signature", message: "No function named %s" };
  } else {
    const __result = (%s as any)(...__args);
    __line = { status: "ok", value: __result === undefined ? null : __result };
  }
} catch (__error) {
  const __e: any = __error;
  __line = { status: "runtime_error", error: __e.name || "Error",
    message: String(__e.message || __e), trace: String(__e.stack || __e) };
}
process.stdout.write("\x1e__SKWTR__ " + JSON.stringify(__line) + "\n");
`, userCode, functionName, functionName, functionName), nil
}
