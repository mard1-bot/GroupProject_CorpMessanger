# Load Testing with k6

This directory contains load testing scripts for the Corp Messenger application using k6.

## Prerequisites

Install k6:
```bash
# macOS
brew install k6

# Linux
sudo gpg -k /usr/share/keyrings/k6-archive-keyring.gpg
curl https://dl.k6.io/keyring.gpg | sudo gpg --dearmor -o /usr/share/keyrings/k6-archive-keyring.gpg

echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

## Running Load Tests

### Basic Load Test

```bash
# Run against local development server
k6 run loadtests/k6/load_test.js

# Run against staging/production server
BASE_URL=https://staging.example.com k6 run loadtests/k6/load_test.js
```

### Load Test Configuration

The `load_test.js` script includes:

- **Stages**: Gradual ramp-up from 10 to 100 users, then ramp-down
- **Thresholds**: 
  - 95% of requests must complete below 500ms
  - Error rate must be less than 5%
- **Test Scenarios**:
  - Health check
  - User profile retrieval
  - Chat list retrieval
  - User search

### Custom Load Tests

You can modify the `options` object in `load_test.js` to customize:

```javascript
export const options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Stay at 100 users
    { duration: '1m', target: 50 },    // Ramp down to 50 users
    { duration: '30s', target: 0 },    // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95th percentile < 500ms
    http_req_failed: ['rate<0.05'],   // Error rate < 5%
  },
};
```

## Interpreting Results

k6 will output:
- **Request duration**: Average, min, max, percentiles
- **Requests per second**: RPS throughout the test
- **Error rate**: Percentage of failed requests
- **Threshold checks**: Whether your performance targets were met

## Best Practices

1. **Start small**: Begin with low user counts and gradually increase
2. **Monitor resources**: Watch CPU, memory, and database connections during tests
3. **Test realistic scenarios**: Use actual user workflows, not just individual endpoints
4. **Run multiple times**: Performance can vary due to caching, database state, etc.
5. **Test in staging**: Never run load tests against production without proper safeguards

## Advanced Scenarios

You can create additional test files for specific scenarios:

- `api_chat_load_test.js` - Focus on chat operations
- `api_message_load_test.js` - Focus on message sending/receiving
- `websocket_load_test.js` - Test WebSocket connections
- `file_upload_load_test.js` - Test file upload performance

Example:
```bash
k6 run loadtests/k6/api_message_load_test.js
```

## CI/CD Integration

Add to your CI/CD pipeline:

```yaml
# GitHub Actions example
- name: Run load tests
  run: |
    k6 run loadtests/k6/load_test.js
```
