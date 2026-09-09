# skwtr-ide-backend

Go backend for the SKWTR IDE. Handles auth, coding challenges, and proxies AI requests to a local Ollama instance.

## Challenges

`GET /v1/challenges/daily` returns the current UTC day's challenge for the authenticated user. Each challenge includes a `solved` boolean derived from that user's completion record.

After the solution has passed the client-side or Judge0 test cases, mark it complete with `POST /v1/challenges/{id}/complete`. Completion records are unique per user and challenge, so retries are idempotent.

SQLite creates the `challenges` and `challenge_completions` tables during migration and seeds the initial daily binary-search challenge when the current day has no challenge.
