# Response Selection Testing Guide

This guide covers how to effectively test APIs using Vanta's response selection functionality, allowing you to control which examples are returned for comprehensive testing scenarios.

## Overview

Vanta's response selection feature enables you to:
- Test specific success/error scenarios consistently
- Validate different user states and permissions
- Test edge cases and boundary conditions
- Automate testing with predictable responses

## Basic Usage

### Single Example Selection

Select a specific example using the `X-Mock-Example` header:

```bash
# Test success scenario
curl -H "X-Mock-Example: success_response" \
     -H "Content-Type: application/json" \
     http://localhost:8080/api/orders

# Test error scenario
curl -H "X-Mock-Example: error_response" \
     -H "Content-Type: application/json" \
     http://localhost:8080/api/orders
```

### Random Example Selection

Use "random" to get different examples on each request:

```bash
curl -H "X-Mock-Example: random" \
     http://localhost:8080/api/users
```

## Common Testing Scenarios

### 1. User State Testing

Test different user roles and states:

```bash
# Test admin user permissions
curl -H "X-Mock-Example: admin_user" \
     http://localhost:8080/api/users/123

# Test regular user permissions
curl -H "X-Mock-Example: regular_user" \
     http://localhost:8080/api/users/123

# Test suspended user
curl -H "X-Mock-Example: suspended_user" \
     http://localhost:8080/api/users/123

# Test user with incomplete profile
curl -H "X-Mock-Example: incomplete_profile" \
     http://localhost:8080/api/users/123
```

### 2. Data Volume Testing

Test different data set sizes:

```bash
# Test empty result set
curl -H "X-Mock-Example: empty_list" \
     http://localhost:8080/api/products

# Test single item
curl -H "X-Mock-Example: single_item" \
     http://localhost:8080/api/products

# Test full page of results
curl -H "X-Mock-Example: full_page" \
     http://localhost:8080/api/products

# Test large dataset
curl -H "X-Mock-Example: large_dataset" \
     http://localhost:8080/api/products
```

### 3. Error Condition Testing

Test various error states:

```bash
# Test validation errors
curl -H "X-Mock-Example: validation_error" \
     -X POST \
     -H "Content-Type: application/json" \
     -d '{"invalid": "data"}' \
     http://localhost:8080/api/users

# Test authentication errors
curl -H "X-Mock-Example: auth_error" \
     http://localhost:8080/api/protected

# Test rate limiting
curl -H "X-Mock-Example: rate_limit_error" \
     http://localhost:8080/api/data

# Test server errors
curl -H "X-Mock-Example: server_error" \
     http://localhost:8080/api/process
```

## Automated Testing Scripts

### Bash Script for Multiple Scenarios

```bash
#!/bin/bash

API_BASE="http://localhost:8080"
ENDPOINT="/api/users"

# Define test scenarios
declare -a scenarios=(
    "admin_user:200"
    "regular_user:200"
    "guest_user:200"
    "suspended_user:403"
    "nonexistent_user:404"
)

echo "Running response selection tests..."

for scenario in "${scenarios[@]}"; do
    IFS=':' read -r example expected_code <<< "$scenario"

    echo "Testing scenario: $example"

    response=$(curl -s -w "HTTPSTATUS:%{http_code}" \
                   -H "X-Mock-Example: $example" \
                   -H "Content-Type: application/json" \
                   "$API_BASE$ENDPOINT/123")

    http_code=$(echo $response | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
    body=$(echo $response | sed -E 's/HTTPSTATUS:[0-9]{3}$//')

    if [ "$http_code" -eq "$expected_code" ]; then
        echo "✅ PASS: $example (HTTP $http_code)"
    else
        echo "❌ FAIL: $example (Expected HTTP $expected_code, got $http_code)"
    fi

    echo "Response: $body"
    echo "---"
done
```

### Python Testing Script

```python
#!/usr/bin/env python3
import requests
import json

API_BASE = "http://localhost:8080"

def test_example_selection(endpoint, examples):
    """Test multiple examples for an endpoint."""
    results = {}

    for example_name, expected_status in examples.items():
        headers = {
            "X-Mock-Example": example_name,
            "Content-Type": "application/json"
        }

        try:
            response = requests.get(f"{API_BASE}{endpoint}", headers=headers)

            success = response.status_code == expected_status
            results[example_name] = {
                "success": success,
                "status_code": response.status_code,
                "expected_status": expected_status,
                "response": response.json() if response.content else None
            }

            status = "✅ PASS" if success else "❌ FAIL"
            print(f"{status}: {example_name} (HTTP {response.status_code})")

        except Exception as e:
            results[example_name] = {
                "success": False,
                "error": str(e)
            }
            print(f"❌ ERROR: {example_name} - {e}")

    return results

# Test user endpoint
user_examples = {
    "admin_user": 200,
    "regular_user": 200,
    "guest_user": 200,
    "suspended_user": 403,
    "deleted_user": 404
}

print("Testing user endpoint examples...")
user_results = test_example_selection("/api/users/123", user_examples)

# Test products endpoint
product_examples = {
    "available_products": 200,
    "out_of_stock": 200,
    "empty_catalog": 200,
    "premium_products": 200
}

print("\nTesting products endpoint examples...")
product_results = test_example_selection("/api/products", product_examples)
```

## Integration with Testing Frameworks

### Jest/Node.js

```javascript
const axios = require('axios');

describe('API Response Selection Tests', () => {
    const API_BASE = 'http://localhost:8080';

    const makeRequest = (endpoint, example) => {
        return axios.get(`${API_BASE}${endpoint}`, {
            headers: {
                'X-Mock-Example': example,
                'Content-Type': 'application/json'
            }
        });
    };

    describe('User Endpoint', () => {
        test('should return admin user data', async () => {
            const response = await makeRequest('/api/users/123', 'admin_user');

            expect(response.status).toBe(200);
            expect(response.data.role).toBe('admin');
            expect(response.data.permissions).toContain('admin');
        });

        test('should return regular user data', async () => {
            const response = await makeRequest('/api/users/123', 'regular_user');

            expect(response.status).toBe(200);
            expect(response.data.role).toBe('user');
            expect(response.data.permissions).not.toContain('admin');
        });

        test('should handle suspended user', async () => {
            try {
                await makeRequest('/api/users/123', 'suspended_user');
            } catch (error) {
                expect(error.response.status).toBe(403);
                expect(error.response.data.error).toContain('suspended');
            }
        });
    });

    describe('Products Endpoint', () => {
        test('should return empty list', async () => {
            const response = await makeRequest('/api/products', 'empty_list');

            expect(response.status).toBe(200);
            expect(response.data).toEqual([]);
        });

        test('should return full catalog', async () => {
            const response = await makeRequest('/api/products', 'full_catalog');

            expect(response.status).toBe(200);
            expect(Array.isArray(response.data)).toBe(true);
            expect(response.data.length).toBeGreaterThan(0);
        });
    });
});
```

### Postman Collection

```json
{
    "info": {
        "name": "Vanta Response Selection Tests",
        "description": "Test suite for response selection functionality"
    },
    "item": [
        {
            "name": "User Tests",
            "item": [
                {
                    "name": "Admin User",
                    "request": {
                        "method": "GET",
                        "header": [
                            {
                                "key": "X-Mock-Example",
                                "value": "admin_user"
                            }
                        ],
                        "url": "{{base_url}}/api/users/123"
                    },
                    "event": [
                        {
                            "listen": "test",
                            "script": {
                                "exec": [
                                    "pm.test('Status code is 200', function () {",
                                    "    pm.response.to.have.status(200);",
                                    "});",
                                    "",
                                    "pm.test('User has admin role', function () {",
                                    "    const jsonData = pm.response.json();",
                                    "    pm.expect(jsonData.role).to.eql('admin');",
                                    "});"
                                ]
                            }
                        }
                    ]
                }
            ]
        }
    ]
}
```

## Best Practices

### 1. Naming Conventions

Use descriptive, consistent names for your examples:

```yaml
examples:
  # Good - descriptive and consistent
  user_admin_active:
    summary: "Active administrator user"
    value: { ... }
  user_regular_suspended:
    summary: "Suspended regular user"
    value: { ... }

  # Avoid - vague or inconsistent
  example1:
    value: { ... }
  test_case:
    value: { ... }
```

### 2. Coverage Strategy

Ensure you cover all important scenarios:

- **Happy path**: Normal successful operations
- **Edge cases**: Boundary conditions, empty results
- **Error conditions**: Various error states and codes
- **User states**: Different roles, permissions, statuses
- **Data variations**: Different data sizes and types

### 3. Test Organization

Structure your tests logically:

```bash
# Group by functionality
curl -H "X-Mock-Example: auth_success" /api/login
curl -H "X-Mock-Example: auth_invalid_password" /api/login
curl -H "X-Mock-Example: auth_user_not_found" /api/login

# Group by user type
curl -H "X-Mock-Example: admin_dashboard" /api/dashboard
curl -H "X-Mock-Example: user_dashboard" /api/dashboard
curl -H "X-Mock-Example: guest_dashboard" /api/dashboard
```

### 4. Documentation

Document your examples in your OpenAPI spec:

```yaml
examples:
  success_case:
    summary: "Successful user creation"
    description: "Returns when user is created successfully with all required fields"
    value:
      id: 123
      username: "newuser"
      email: "user@example.com"
      status: "active"
```

## Troubleshooting

### Example Not Found

If an example isn't found, Vanta falls back to default behavior:

```bash
# This will fall back to first available example or generated data
curl -H "X-Mock-Example: nonexistent_example" /api/data
```

### Random Selection Not Working

Ensure you're using the exact string "random":

```bash
# Correct
curl -H "X-Mock-Example: random" /api/data

# Incorrect
curl -H "X-Mock-Example: Random" /api/data
curl -H "X-Mock-Example: RANDOM" /api/data
```

### Configuration Issues

Check your Vanta configuration:

```yaml
mock:
  prefer_examples: true    # Must be true to use examples
  example_strategy: "header"  # Should be "header" for X-Mock-Example support
```

This guide provides a comprehensive approach to testing APIs using Vanta's response selection functionality, enabling thorough and predictable testing of various scenarios.