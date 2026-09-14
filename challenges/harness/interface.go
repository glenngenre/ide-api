package harness

type HarnessBuilder interface {
	Build(functionName string, userCode string) (string, error)
}
