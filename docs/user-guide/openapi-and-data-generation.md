# OpenAPI Support & Data Generation

Vanta parses OpenAPI 3.x specs and generates realistic responses for registered routes.

## Response Selection

Vanta provides advanced functionality to select between different response examples defined in OpenAPI specifications. This feature enables controlled testing of various scenarios.

### Selection Methods

#### 1. HTTP Header Selection - `X-Mock-Example`

The primary method for selecting specific examples is through the `X-Mock-Example` HTTP header:

```bash
# Select a specific example
curl -H "X-Mock-Example: user_success" http://localhost:8080/users/1

# Get random example
curl -H "X-Mock-Example: random" http://localhost:8080/users/1

# Select from multiple available options
curl -H "X-Mock-Example: large_list" http://localhost:8080/pets
```

#### 2. Configuration-based Selection

Configure default selection strategy in your YAML configuration:

```yaml
mock:
  example_strategy: "header"     # "header", "first", "random"
  default_example: "success"     # Default example when no header specified
  prefer_examples: true          # Prioritize examples over auto-generation
```

### Selection Strategies

- **"header"** (default): Reads `X-Mock-Example` header value, falls back to `default_example`
- **"first"**: Always selects the first available example, ignores headers
- **"random"**: Randomly selects from available examples (respects seed for reproducibility)

### Fallback Logic

Vanta implements robust fallback behavior:

1. Does requested example exist in header? → Use that example
2. Header = "random"? → Select random example
3. Single `example` exists? → Use single example
4. Multiple `examples` exist? → Use first available example
5. No examples available → Generate data automatically

### OpenAPI Example Structure

Define multiple examples in your OpenAPI spec:

```yaml
/users/{userId}:
  get:
    responses:
      '200':
        content:
          application/json:
            examples:
              admin_user:
                summary: "Administrator user"
                value:
                  id: 1
                  username: "admin"
                  role: "admin"
                  permissions: ["read", "write", "admin"]
              regular_user:
                summary: "Regular user"
                value:
                  id: 2
                  username: "user"
                  role: "user"
                  permissions: ["read"]
              guest_user:
                summary: "Guest user"
                value:
                  id: 3
                  username: "guest"
                  role: "guest"
                  permissions: []
```

## How responses are generated
- Preferred examples: If a response has an example, it is returned based on selection strategy.
- Enums: A random enum value is selected (deterministic with seed).
- Strings: Length constraints respected; patterns are approximated with letters.
- Numbers/Integers: Respect minimum/maximum when provided.
- Objects: Required properties always included; optional ones included probabilistically.
- Arrays: Size determined by min/max and default array size.

## Determinism
Set `mock.seed` to a fixed value to get repeatable responses across runs. Without a seed, the generator seeds from current time.

## Locale
`mock.locale` is stored and can be used by custom generators; default is `en`.

## Parameterized paths
Paths like `/pets/{petId}` are matched against actual requests. Path parameters are extracted and used for routing.

## Tips
- Provide concrete examples in your OpenAPI spec for stable outputs where needed.
- Use enums to constrain generated values.
- Keep `max_depth` reasonable to avoid deep recursion in nested schemas.

