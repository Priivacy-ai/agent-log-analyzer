# Load Shedding

The API can reject new analysis-session creation before issuing an upload token when the analysis queue is too deep.

```bash
CLAUDE_ANALYZER_MAX_QUEUE_DEPTH=1000
```

Behavior:

- `0` or unset disables queue-depth rejection.
- When queue depth is greater than or equal to the threshold, `POST /api/analysis-sessions` returns `503`.
- The response includes `Retry-After: 60`.
- The API checks queue depth before issuing a one-time upload token.

This is the first production backpressure control. In AWS mode, depth is read from SQS approximate visible and in-flight message counts. In local mode, depth is pending plus processing job files.
