import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m', target: 10 },    // Stay at 10 users
    { duration: '30s', target: 50 },   // Ramp up to 50 users
    { duration: '1m', target: 50 },    // Stay at 50 users
    { duration: '30s', target: 100 },  // Ramp up to 100 users
    { duration: '1m', target: 100 },   // Stay at 100 users
    { duration: '30s', target: 0 },    // Ramp down to 0
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must complete below 500ms
    errors: ['rate<0.1'],              // Error rate must be below 10%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
  // Register a test user
  const email = `loadtest-${Math.random().toString(36).substring(7)}@test.com`;
  const password = 'TestPassword123!';
  
  const registerRes = http.post(`${BASE_URL}/api/v1/auth/register`, JSON.stringify({
    email: email,
    password: password,
    name: 'Load Test User',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (registerRes.status !== 201 && registerRes.status !== 200) {
    console.error('Registration failed:', registerRes.status, registerRes.body);
  }

  // Login to get token
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    email: email,
    password: password,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (loginRes.status !== 200) {
    console.error('Login failed:', loginRes.status, loginRes.body);
    return { token: null };
  }

  const token = loginRes.json('token');
  return { token };
}

export default function (data) {
  if (!data.token) {
    console.error('No token available, skipping test');
    return;
  }

  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Test 1: Get user profile
  const profileRes = http.get(`${BASE_URL}/api/v1/users/me`, { headers });
  check(profileRes, {
    'profile status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(1);

  // Test 2: Get chats list
  const chatsRes = http.get(`${BASE_URL}/api/v1/chats`, { headers });
  check(chatsRes, {
    'chats status is 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(1);

  // Test 3: Send a message (if there are chats)
  if (chatsRes.status === 200 && chatsRes.json().length > 0) {
    const chatId = chatsRes.json()[0].id;
    const messageRes = http.post(`${BASE_URL}/api/v1/chats/${chatId}/messages`, JSON.stringify({
      content: 'Load test message',
      type: 'text',
    }), { headers });
    check(messageRes, {
      'message status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    }) || errorRate.add(1);
  }

  sleep(2);
}
