# OpenAPI Support & Data Generation

Vanta parses OpenAPI 3.x specs and generates realistic responses for registered routes.

## How responses are generated
- Preferred examples: If a response has an example, it is returned.
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

