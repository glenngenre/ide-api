package harness

type HarnessBuilder interface {
	Build(functionName string, userCode string, casesJSON string) (string, error)
}
