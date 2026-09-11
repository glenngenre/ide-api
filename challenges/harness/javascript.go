package harness

import "fmt"

type JavaScriptBuilder struct{}

func (b *JavaScriptBuilder) WrapWithStdin(functionName, userCode string) string {
	return fmt.Sprintf(`%s

;(async function(fn) {
  const process = require('process');
  const write = process.stdout.write.bind(process.stdout);
  const args = JSON.parse(require('fs').readFileSync(0, 'utf8'));
  const result = await fn(...args);
  const output = JSON.stringify(result === undefined ? null : result);
  if (output === undefined) throw new TypeError('Return value is not JSON serializable');
  write(%q + output + '\n');
})(%s).catch((error) => {
  const process = require('process');
  process.stderr.write(String(error && error.stack || error) + '\n');
  process.exitCode = 1;
});
`, userCode, ResultPrefix, functionName)
}
