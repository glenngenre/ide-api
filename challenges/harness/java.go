package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JavaBuilder struct{}

func (b *JavaBuilder) Build(functionName string, userCode string, casesJSON string) (string, error) {
	// Hoist imports from user code
	var imports []string
	var userCodeLines []string

	for _, line := range strings.Split(userCode, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			imports = append(imports, line)
		} else {
			userCodeLines = append(userCodeLines, line)
		}
	}

	hoistedCode := strings.Join(userCodeLines, "\n")
	hoistedImports := strings.Join(imports, "\n")
	if hoistedImports != "" {
		hoistedImports += "\n"
	}

	// Parse cases for direct generation
	mainCalls := generateJavaMainCalls(casesJSON, functionName)

	harness := fmt.Sprintf(`import java.util.*;
import java.io.*;
import java.util.function.Supplier;
%s
public class Main {
%s

    private static void __run(PrintStream out, int i, Supplier<Object> fn) {
        ByteArrayOutputStream buf = new ByteArrayOutputStream();
        System.setOut(new PrintStream(buf, true));
        String line;
        try {
            Object r = fn.get();
            line = "{\"i\":" + i + ",\"ok\":true,\"out\":" + __json(r) + ",\"stdout\":" + __json(buf.toString()) + "}";
        } catch (Throwable t) {
            StringWriter sw = new StringWriter();
            t.printStackTrace(new PrintWriter(sw));
            line = "{\"i\":" + i + ",\"ok\":false,\"err\":" + __json(sw.toString()) + ",\"stdout\":" + __json(buf.toString()) + "}";
        } finally {
            System.setOut(out);
        }
        out.println("__SKWTR__ " + line);
    }

    private static String __json(Object v) {
        if (v == null) return "null";
        if (v instanceof String) {
            return "\"" + escapeJsonString((String) v) + "\"";
        }
        if (v instanceof Boolean) return v.toString();
        if (v instanceof Number) {
            String s = v.toString();
            if (s.endsWith(".0") && v instanceof Double) {
                return s.substring(0, s.length() - 2);
            }
            return s;
        }
        if (v instanceof int[]) {
            int[] arr = (int[]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                sb.append(arr[i]);
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof long[]) {
            long[] arr = (long[]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                sb.append(arr[i]);
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof double[]) {
            double[] arr = (double[]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                double d = arr[i];
                if (d == (long) d) {
                    sb.append((long) d);
                } else {
                    sb.append(d);
                }
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof boolean[]) {
            boolean[] arr = (boolean[]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                sb.append(arr[i]);
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof String[]) {
            String[] arr = (String[]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                sb.append(__json(arr[i]));
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof int[][]) {
            int[][] arr = (int[][]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                sb.append(__json(arr[i]));
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof String[][]) {
            String[][] arr = (String[][]) v;
            StringBuilder sb = new StringBuilder("[");
            for (int i = 0; i < arr.length; i++) {
                if (i > 0) sb.append(",");
                sb.append(__json(arr[i]));
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof Collection) {
            Collection<?> c = (Collection<?>) v;
            StringBuilder sb = new StringBuilder("[");
            boolean first = true;
            for (Object item : c) {
                if (!first) sb.append(",");
                sb.append(__json(item));
                first = false;
            }
            sb.append("]");
            return sb.toString();
        }
        if (v instanceof Map) {
            Map<?, ?> m = (Map<?, ?>) v;
            StringBuilder sb = new StringBuilder("{");
            boolean first = true;
            for (Map.Entry<?, ?> e : m.entrySet()) {
                if (!first) sb.append(",");
                sb.append(__json(e.getKey().toString())).append(":");
                sb.append(__json(e.getValue()));
                first = false;
            }
            sb.append("}");
            return sb.toString();
        }
        return "null";
    }

    private static String escapeJsonString(String s) {
        StringBuilder sb = new StringBuilder();
        for (char c : s.toCharArray()) {
            switch (c) {
                case '"': sb.append("\\\""); break;
                case '\\': sb.append("\\\\"); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                case '\b': sb.append("\\b"); break;
                case '\f': sb.append("\\f"); break;
                default:
                    if (c < 32) {
                        sb.append(String.format("\\u%%04x", (int) c));
                    } else {
                        sb.append(c);
                    }
            }
        }
        return sb.toString();
    }

    public static void main(String[] args) {
        PrintStream out = System.out;
        Main m = new Main();
%s
    }
}
`, hoistedImports, hoistedCode, mainCalls)

	return harness, nil
}

func generateJavaMainCalls(casesJSON string, functionName string) string {
	var cases [][]any
	if err := json.Unmarshal([]byte(casesJSON), &cases); err != nil {
		return "        // Error parsing test cases\n"
	}

	var sb strings.Builder
	for i, tc := range cases {
		fmt.Fprintf(&sb, "        __run(out, %d, () -> m.%s(", i, functionName)

		for j, arg := range tc {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(generateJavaLiteral(arg))
		}

		sb.WriteString("));\n")
	}

	return sb.String()
}

func generateJavaLiteral(v any) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dL", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case string:
		escaped := strings.NewReplacer(
			"\\", "\\\\",
			"\"", "\\\"",
			"\n", "\\n",
			"\r", "\\r",
			"\t", "\\t",
		).Replace(val)
		return fmt.Sprintf("\"%s\"", escaped)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		return generateJavaArray(val)
	case map[string]any:
		return generateJavaMap(val)
	default:
		return "null"
	}
}

func generateJavaArray(arr []any) string {
	if len(arr) == 0 {
		return "new Object[0]"
	}

	firstType := inferJavaType(arr[0])

	var sb strings.Builder
	fmt.Fprintf(&sb, "new %s[]{", firstType)

	for i, item := range arr {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(generateJavaLiteral(item))
	}

	sb.WriteString("}")
	return sb.String()
}

func inferJavaType(v any) string {
	switch v.(type) {
	case float64:
		return "int"
	case string:
		return "String"
	case bool:
		return "boolean"
	case []any:
		return "Object"
	default:
		return "Object"
	}
}

func generateJavaMap(m map[string]any) string {
	sb := strings.Builder{}
	sb.WriteString("new java.util.HashMap<String, Object>() {{")

	first := true
	for k, v := range m {
		if !first {
			sb.WriteString("; ")
		}
		fmt.Fprintf(&sb, "put(\"%s\", %s)", k, generateJavaLiteral(v))
		first = false
	}

	sb.WriteString("}}")
	return sb.String()
}
