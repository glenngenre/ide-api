package harness

import "fmt"

type JavaScriptBuilder struct{}

func (b *JavaScriptBuilder) Build(functionName string, userCode string, casesJSON string) (string, error) {
	harness := fmt.Sprintf(`%s

;(function () {
  const __cases = %s;
  const __write = (s) => process.stdout.write(s + "\n");
  for (let i = 0; i < __cases.length; i++) {
    const buf = [];
    const origLog = console.log;
    console.log = (...a) => { buf.push(a.map(String).join(" ")); };
    let line;
    try {
      const r = (%s)(...__cases[i]);
      line = { i, ok: true, out: r === undefined ? null : r, stdout: buf.join("\n") };
    } catch (e) {
      line = { i, ok: false, err: (e && e.stack) || String(e), stdout: buf.join("\n") };
    } finally {
      console.log = origLog;
    }
    __write("__SKWTR__ " + JSON.stringify(line));
  }
})();
`, userCode, casesJSON, functionName)

	return harness, nil
}
