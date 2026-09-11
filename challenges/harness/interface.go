package harness

const ResultPrefix = "\x1e__SKWTR_RESULT__ "

type HarnessBuilder interface {
	WrapWithStdin(functionName string, userCode string) string
}
