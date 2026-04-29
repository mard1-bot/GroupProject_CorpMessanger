import http from 'k6/http';
import { check, sleep } from 'k6';

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Stay at 100 users
    { duration: '1m', target: 50 },    // Ramp down to 50 users
    { duration: '30s', target: 0 },    // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must complete below 500ms
    http_req_failed: ['rate<0.05'],   // Error rate must be less than 5%
  },
};

// Test data
const testUser = {
  email: `loadtest-${Math.random()}@example.com`,
  password: 'SecurePass123!',
};

export function setup() {
  // Setup: Register a test user
  const registerRes = http.post(
    `${BASE_URL}/api/register`,
    JSON.stringify(testUser),
    {
      headers: { 'Content-Type': 'application/json' },
    }
  );
  
  // Login to get token
  const loginRes = http.post(
    `${BASE_URL}/api/login`,
    JSON.stringify({
      email: testUser.email,
      password: testUser.password,
    }),
    {
      headers: { 'Content-Type': 'application/json' },
    }
  );
  
  const token = JSON.parse(loginRes.body).token;
  return { token };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Test 1: Health check
  let healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health status is 200': (r) => r.status === 200,
  });

  // Test 2: Get current user
  let userRes = http.get(`${BASE_URL}/api/users/me`, { headers });
  check(userRes, {
    'user profile status is 200': (r) => r.status === 200,
  });

  // Test 3: Get user chats
  let chatsRes = http.get(`${BASE_URL}/api/chats`, { headers });
  check(chatsRes, {
    'chats status is 200': (r) => r.status === 200,
  });

  // Test 4: Search users
  let searchRes = http.get(`${BASE_URL}/api/users?search=test`, { headers });
  check(searchRes, {
    'search status is 200': (r) => r.status === 200,
  });

  sleep(1);
}

export function teardown(data) {
  // Cleanup: Delete test user if needed
  // This would require an endpoint to delete account
}
