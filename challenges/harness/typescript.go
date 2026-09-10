package harness

import "fmt"

type TypeScriptBuilder struct{}

func (b *TypeScriptBuilder) Build(functionName string, userCode string, casesJSON string) (string, error) {
	harness := fmt.Sprintf(`%s

;(function () {
  const __cases: any[][] = %s;
  const __write = (s: string) => process.stdout.write(s + "\n");
  for (let i = 0; i < __cases.length; i++) {
    const buf: string[] = [];
    const origLog = console.log;
    console.log = (...a: any[]) => { buf.push(a.map(String).join(" ")); };
    let line: any;
    try {
      const r = (%s as any)(...__cases[i]);
      line = { i, ok: true, out: r === undefined ? null : r, stdout: buf.join("\n") };
    } catch (e) {
      line = { i, ok: false, err: (e && (e as any).stack) || String(e), stdout: buf.join("\n") };
    } finally {
      console.log = origLog;
    }
    __write("__SKWTR__ " + JSON.stringify(line));
  }
})();
`, userCode, casesJSON, functionName)

	return harness, nil
}
