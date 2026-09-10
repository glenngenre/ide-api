package challenges

import (
	"encoding/json"
)

func Equal(expected, actual json.RawMessage) bool {
	var expVal, actVal any

	if err := json.Unmarshal(expected, &expVal); err != nil {
		return false
	}
	if err := json.Unmarshal(actual, &actVal); err != nil {
		return false
	}

	return equalValues(expVal, actVal)
}

func equalValues(expected, actual any) bool {
	if expected == nil && actual == nil {
		return true
	}
	if expected == nil || actual == nil {
		return false
	}

	if expStr, ok := expected.(string); ok {
		if actStr, ok := actual.(string); ok {
			return expStr == actStr
		}
		return false
	}

	if expBool, ok := expected.(bool); ok {
		if actBool, ok := actual.(bool); ok {
			return expBool == actBool
		}
		return false
	}

	if expNum, ok := expected.(float64); ok {
		if actNum, ok := actual.(float64); ok {
			return numericEqual(expNum, actNum)
		}
		return false
	}

	if expArr, ok := expected.([]any); ok {
		if actArr, ok := actual.([]any); ok {
			if len(expArr) != len(actArr) {
				return false
			}
			for i := range expArr {
				if !equalValues(expArr[i], actArr[i]) {
					return false
				}
			}
			return true
		}
		return false
	}

	if expObj, ok := expected.(map[string]any); ok {
		if actObj, ok := actual.(map[string]any); ok {
			if len(expObj) != len(actObj) {
				return false
			}
			for k, expV := range expObj {
				if actV, ok := actObj[k]; !ok || !equalValues(expV, actV) {
					return false
				}
			}
			return true
		}
		return false
	}

	return false
}

func numericEqual(expected, actual float64) bool {
	if expected == actual {
		return true
	}

	expIsInt := expected == float64(int64(expected))
	actIsInt := actual == float64(int64(actual))

	if expIsInt && actIsInt {
		return int64(expected) == int64(actual)
	}

	const epsilon = 1e-9
	diff := expected - actual
	if diff < 0 {
		diff = -diff
	}

	if diff < epsilon {
		return true
	}

	maxAbs := expected
	if actual > maxAbs {
		maxAbs = actual
	}
	if maxAbs < 0 {
		maxAbs = -maxAbs
	}

	if maxAbs == 0 {
		return diff == 0
	}

	return diff/maxAbs < epsilon
}

func BuildCaseResults(
	harnessResults []HarnessResult,
	expectedOutputs []json.RawMessage,
	testCasesHidden []bool,
	isOverride bool,
) []any {
	results := make([]any, 0, len(expectedOutputs))

	for i := 0; i < len(expectedOutputs); i++ {
		result := map[string]any{
			"index": i,
		}

		hidden := false
		if i < len(testCasesHidden) {
			hidden = testCasesHidden[i]
		}
		result["hidden"] = hidden

		var harnessResult *HarnessResult
		for j := range harnessResults {
			if harnessResults[j].Index == i {
				harnessResult = &harnessResults[j]
				break
			}
		}

		if harnessResult == nil {
			passed := false
			result["passed"] = passed
			result["error"] = "Test case not executed"
		} else if !harnessResult.Ok {
			// Test case had an error
			passed := false
			result["passed"] = passed
			result["error"] = harnessResult.Err

			if !hidden {
				var inputArray []json.RawMessage
				if err := json.Unmarshal(harnessResult.Out, &inputArray); err == nil {
					result["input"] = inputArray
				}
				result["stdout"] = harnessResult.Stdout
			}
		} else {
			passed := Equal(expectedOutputs[i], harnessResult.Out)

			if isOverride && expectedOutputs[i] == nil {
				result["passed"] = nil
			} else {
				result["passed"] = passed
			}

			if !hidden {
				result["input"] = unmarshalInputArray(harnessResult.Out)
				result["expected"] = json.RawMessage(expectedOutputs[i])
				result["actual"] = harnessResult.Out
				result["stdout"] = harnessResult.Stdout
			}
		}

		results = append(results, result)
	}

	return results
}

func unmarshalInputArray(data json.RawMessage) []json.RawMessage {
	var result []json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

func CountPassed(results []any) int {
	count := 0
	for _, r := range results {
		if m, ok := r.(map[string]any); ok {
			if passed, ok := m["passed"].(bool); ok && passed {
				count++
			}
		}
	}
	return count
}
