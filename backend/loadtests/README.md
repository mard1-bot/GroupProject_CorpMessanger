# Load Testing

This directory contains load testing scripts using k6.

## Prerequisites

Install k6:
```bash
# On macOS
brew install k6

# On Linux
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

## Running Load Tests

### Basic Load Test

Tests user registration, login, and basic API operations under load:

```bash
# Run against local backend
BASE_URL=http://localhost:8080 k6 run basic_load_test.js

# Run against production backend
BASE_URL=https://your-api.com k6 run basic_load_test.js

# Run with custom configuration
k6 run --out json=results.json basic_load_test.js
```

### Test Scenarios

The basic load test includes:
1. User registration
2. User login
3. Get user profile
4. Get chats list
5. Send messages

### Load Test Configuration

The test is configured with the following stages:
- Ramp up to 10 users over 30 seconds
- Stay at 10 users for 1 minute
- Ramp up to 50 users over 30 seconds
- Stay at 50 users for 1 minute
- Ramp up to 100 users over 30 seconds
- Stay at 100 users for 1 minute
- Ramp down to 0 users over 30 seconds

### Thresholds

The test will fail if:
- 95th percentile response time exceeds 500ms
- Error rate exceeds 10%

## Creating Custom Load Tests

To create a custom load test:

1. Create a new JavaScript file in this directory
2. Use the k6 API to define your test scenarios
3. Configure stages, thresholds, and metrics
4. Run with `k6 run your_test.js`

For more information, see the [k6 documentation](https://k6.io/docs/).
