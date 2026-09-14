package harness

import (
	"fmt"
	"strings"
)

type JavaBuilder struct{}

func (b *JavaBuilder) Build(functionName string, userCode string) (string, error) {
	var imports []string
	var body []string
	for _, line := range strings.Split(userCode, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			imports = append(imports, line)
		} else {
			body = append(body, line)
		}
	}

	return fmt.Sprintf(`import java.lang.reflect.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import com.google.gson.*;
%s
public class Main {

    private static final PrintStream ORIG_OUT = System.out;

    // User code
%s

    public static void main(String[] args) {
        Gson gson = new Gson();
        try {
            JsonElement parsed = JsonParser.parseReader(
                new InputStreamReader(System.in, StandardCharsets.UTF_8)
            );
            if (parsed == null || !parsed.isJsonArray()) {
                emit(gson, "bad_input", "stdin must be a JSON array of arguments");
                return;
            }
            JsonArray input = parsed.getAsJsonArray();
            String fn = "%s";
            Method target = null;
            for (Method method : Main.class.getDeclaredMethods()) {
                if (method.getName().equals(fn) && method.getParameterCount() == input.size()) {
                    if (target != null) {
                        emit(gson, "bad_signature", "Ambiguous method: " + fn + "/" + input.size());
                        return;
                    }
                    target = method;
                }
            }
            if (target == null) {
                emit(gson, "bad_signature",
                    "No method named " + fn + " taking " + input.size() + " argument(s)");
                return;
            }

            Type[] parameterTypes = target.getGenericParameterTypes();
            Object[] parameters = new Object[parameterTypes.length];
            for (int i = 0; i < parameterTypes.length; i++) {
                parameters[i] = gson.fromJson(input.get(i), parameterTypes[i]);
            }

            Object receiver = Modifier.isStatic(target.getModifiers()) ? null : new Main();
            try {
                emit(gson, "ok", target.invoke(receiver, parameters));
            } catch (InvocationTargetException invocationError) {
                emitThrowable(gson, invocationError.getCause() != null
                    ? invocationError.getCause() : invocationError);
            }
        } catch (Throwable error) {
            emitThrowable(gson, error);
        }
    }

    private static void emitThrowable(Gson gson, Throwable error) {
        StringWriter stack = new StringWriter();
        error.printStackTrace(new PrintWriter(stack));
        JsonObject result = new JsonObject();
        result.addProperty("status", "runtime_error");
        result.addProperty("error", error.getClass().getName());
        result.addProperty("message", String.valueOf(error.getMessage()));
        result.addProperty("trace", stack.toString());
        emitRaw(gson, result);
    }

    private static void emit(Gson gson, String status, Object value) {
        JsonObject result = new JsonObject();
        result.addProperty("status", status);
        result.add("value", gson.toJsonTree(value));
        emitRaw(gson, result);
    }

    private static void emitRaw(Gson gson, JsonObject result) {
        ORIG_OUT.print('\u001e');
        ORIG_OUT.print("__SKWTR__ ");
        ORIG_OUT.println(gson.toJson(result));
        ORIG_OUT.flush();
    }
}
`, strings.Join(imports, "\n"), strings.Join(body, "\n"), strings.ReplaceAll(functionName, `"`, `\"`)), nil
}
