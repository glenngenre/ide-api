package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JavaBuilder struct{}

func (b *JavaBuilder) WrapWithStdin(functionName, userCode string) string {
	var imports, code []string
	for _, line := range strings.Split(userCode, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import\t") {
			imports = append(imports, line)
		} else {
			code = append(code, line)
		}
	}

	methodName, _ := json.Marshal(functionName)
	resultPrefix, _ := json.Marshal(ResultPrefix)

	return fmt.Sprintf(`import java.util.*;
import com.google.gson.*;
%s

public class Main {
%s

    public static void main(String[] args) {
        final java.io.PrintStream out = System.out;
        final java.io.PrintStream err = System.err;
        try {
            Gson gson = new Gson();
            JsonArray input = gson.fromJson(
                new java.io.InputStreamReader(System.in, java.nio.charset.StandardCharsets.UTF_8),
                JsonArray.class);
            if (input == null) {
                throw new IllegalArgumentException("Expected a JSON array of arguments on stdin");
            }

            String functionName = %s;
            java.lang.reflect.Method target = null;
            for (java.lang.reflect.Method method : Main.class.getDeclaredMethods()) {
                if (method.getName().equals(functionName) && method.getParameterCount() == input.size()) {
                    if (target != null) {
                        throw new IllegalArgumentException("Ambiguous method: " + functionName
                            + " with " + input.size() + " arguments");
                    }
                    target = method;
                }
            }
            if (target == null) {
                throw new NoSuchMethodException("No method named " + functionName
                    + " with " + input.size() + " arguments");
            }

            java.lang.reflect.Type[] parameterTypes = target.getGenericParameterTypes();
            Object[] params = new Object[parameterTypes.length];
            for (int i = 0; i < parameterTypes.length; i++) {
                params[i] = gson.fromJson(input.get(i), parameterTypes[i]);
            }
            Object receiver = java.lang.reflect.Modifier.isStatic(target.getModifiers()) ? null : new Main();
            Object result = target.invoke(receiver, params);
            // Keep the marker and JSON together, even after unterminated user output or setOut.
            out.print(%s + gson.toJson(result) + "\n");
            out.flush();
        } catch (Throwable failure) {
            if (failure instanceof java.lang.reflect.InvocationTargetException && failure.getCause() != null) {
                failure = failure.getCause();
            }
            failure.printStackTrace(err);
            System.exit(1);
        }
    }
}
`, strings.Join(imports, "\n"), strings.Join(code, "\n"), methodName, resultPrefix)
}
