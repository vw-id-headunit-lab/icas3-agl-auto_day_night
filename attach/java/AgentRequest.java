package lab.daynight.attach;

import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.util.Arrays;

/** Reserve the result before touching HMI state, keeping the same open file until completion. */
final class AgentRequest implements AutoCloseable {
    final String action;
    private final OutputStream output;

    private AgentRequest(String action, OutputStream output) {
        this.action = action;
        this.output = output;
    }

    static AgentRequest open(String options) throws IOException {
        String[] fields = options == null ? new String[0] : options.split("\\|", -1);
        if (fields.length != 2 || !Arrays.asList("install", "remove", "status").contains(fields[0]))
            throw new IllegalArgumentException("Expected action|absolute-result-file");
        Path path = Paths.get(fields[1]);
        if (!path.isAbsolute()) throw new IOException("Result path must be absolute");
        return new AgentRequest(fields[0], Files.newOutputStream(path,
            StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE));
    }

    void write(String json) throws IOException {
        output.write(json.getBytes(StandardCharsets.UTF_8));
        output.flush();
    }

    public void close() throws IOException { output.close(); }

    static String quote(String value) {
        StringBuilder escaped = new StringBuilder("\"");
        for (int i = 0; i < value.length(); i++) {
            char ch = value.charAt(i);
            if (ch == '\\' || ch == '"') escaped.append('\\').append(ch);
            else if (ch < 0x20) escaped.append(String.format("\\u%04x", (int)ch));
            else escaped.append(ch);
        }
        return escaped.append('"').toString();
    }
}
