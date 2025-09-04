# Plan de Implementación Detallado - OpenAPI Mocker

## FASE 1: Configuración del Proyecto (Semanas 1-2)

### 1.1 Inicialización del Proyecto Base

#### Tarea 1.1.1: Estructura de Directorios
- **Acción**: Crear directorio principal `openapi-mocker/`
- **Comando**: `mkdir openapi-mocker && cd openapi-mocker`
- **Estructura exacta a crear**:
  ```
  openapi-mocker/
  ├── cmd/mocker/
  ├── pkg/api/
  ├── pkg/openapi/
  ├── pkg/config/
  ├── pkg/plugins/
  ├── pkg/recorder/
  ├── pkg/chaos/
  ├── pkg/templates/
  ├── pkg/cli/
  ├── internal/cache/
  ├── internal/hotreload/
  ├── internal/metrics/
  ├── internal/utils/
  ├── test/fixtures/
  ├── test/integration/
  ├── test/benchmarks/
  ├── test/examples/
  ├── scripts/
  └── docs/
  ```

#### Tarea 1.1.2: Inicialización Go Module
- **Acción**: Ejecutar `go mod init github.com/usuario/openapi-mocker`
- **Archivo resultante**: `go.mod` con módulo base
- **Verificar**: Comando `go mod tidy` ejecuta sin errores

#### Tarea 1.1.3: Dependencias Core
- **Acción**: Agregar dependencias específicas al `go.mod`
- **Dependencias exactas**:
  ```go
  github.com/valyala/fasthttp v1.51.0
  github.com/spf13/cobra v1.8.0
  github.com/spf13/viper v1.18.2
  github.com/getkin/kin-openapi v0.120.0
  github.com/charmbracelet/bubbletea v0.25.0
  github.com/charmbracelet/lipgloss v0.9.1
  go.uber.org/zap v1.26.0
  github.com/prometheus/client_golang v1.18.0
  github.com/brianvoe/gofakeit/v6 v6.23.2
  github.com/fsnotify/fsnotify v1.7.0
  github.com/stretchr/testify v1.8.4
  ```
- **Comando**: `go get <cada-dependencia>`

#### Tarea 1.1.4: Makefile Base
- **Acción**: Crear `Makefile` con targets específicos
- **Contenido exacto**:
  ```makefile
  .PHONY: build test clean install lint fmt vet
  
  build:
  	go build -o bin/mocker ./cmd/mocker
  
  test:
  	go test ./...
  
  clean:
  	rm -rf bin/
  
  install:
  	go install ./cmd/mocker
  
  lint:
  	golangci-lint run
  
  fmt:
  	go fmt ./...
  
  vet:
  	go vet ./...
  ```

### 1.2 CLI Framework Base

#### Tarea 1.2.1: Main Entry Point
- **Archivo**: `cmd/mocker/main.go`
- **Contenido específico**: 
  - Importar cobra y zap
  - Crear rootCmd con subcomandos: start, config, version
  - Inicializar logger con zap
  - Manejar signals (SIGINT, SIGTERM)
- **Función main()**: Ejecutar rootCmd.Execute()
- **Validar**: `go run cmd/mocker/main.go --help` muestra comandos

#### Tarea 1.2.2: Comando Start
- **Archivo**: `cmd/mocker/start.go` 
- **Flags requeridos**:
  - `--spec` (string): Path al archivo OpenAPI
  - `--port` (int): Puerto del servidor (default: 8080)
  - `--host` (string): Host del servidor (default: "0.0.0.0")
  - `--config` (string): Path al archivo de configuración
- **Acción**: Parsear spec y iniciar servidor HTTP
- **Validar**: `mocker start --help` muestra todas las flags

#### Tarea 1.2.3: Comando Config
- **Archivo**: `cmd/mocker/config.go`
- **Subcomandos**:
  - `config init`: Crear configuración por defecto
  - `config validate`: Validar archivo de configuración
  - `config edit`: Abrir editor interactivo
- **Validar**: `mocker config init` crea `mocker.yaml`

#### Tarea 1.2.4: Comando Version
- **Archivo**: `cmd/mocker/version.go`
- **Variables de build**: version, commit, buildTime
- **Output formato**: `mocker version 1.0.0 (commit: abc123, built: 2024-01-01)`
- **Validar**: `mocker version` muestra información correcta

### 1.3 Configuración Base

#### Tarea 1.3.1: Estructura de Configuración
- **Archivo**: `pkg/config/config.go`
- **Struct exacta**:
  ```go
  type Config struct {
      Server    ServerConfig    `yaml:"server"`
      Chaos     ChaosConfig     `yaml:"chaos"`
      Plugins   []PluginConfig  `yaml:"plugins"`
      Logging   LoggingConfig   `yaml:"logging"`
      Metrics   MetricsConfig   `yaml:"metrics"`
  }
  
  type ServerConfig struct {
      Port            int           `yaml:"port"`
      Host            string        `yaml:"host"`
      ReadTimeout     time.Duration `yaml:"read_timeout"`
      WriteTimeout    time.Duration `yaml:"write_timeout"`
      MaxConnsPerIP   int           `yaml:"max_conns_per_ip"`
      MaxRequestSize  string        `yaml:"max_request_size"`
      Concurrency     int           `yaml:"concurrency"`
      ReusePort       bool          `yaml:"reuse_port"`
  }
  ```

#### Tarea 1.3.2: Carga de Configuración
- **Archivo**: `pkg/config/loader.go`
- **Función**: `Load(configPath string) (*Config, error)`
- **Soporte**: YAML, JSON, environment variables
- **Precedencia**: CLI flags > env vars > config file > defaults
- **Usar**: Viper para carga y binding

#### Tarea 1.3.3: Validación de Configuración
- **Archivo**: `pkg/config/validation.go`
- **Función**: `Validate(cfg *Config) error`
- **Validaciones**:
  - Puerto entre 1-65535
  - Host válido (IP o hostname)
  - Timeouts > 0
  - MaxRequestSize parseable
- **Return**: Lista de errores de validación

#### Tarea 1.3.4: Configuración por Defecto
- **Archivo**: `pkg/config/defaults.go`
- **Función**: `DefaultConfig() *Config`
- **Valores por defecto**:
  - Port: 8080
  - Host: "0.0.0.0"
  - ReadTimeout: 30s
  - WriteTimeout: 30s
  - Concurrency: 256000
- **Template**: Generar `mocker.yaml` con estos defaults

### 1.4 Parser OpenAPI Base

#### Tarea 1.4.1: Interface Parser
- **Archivo**: `pkg/openapi/parser.go`
- **Interface exacta**:
  ```go
  type SpecParser interface {
      Parse(data []byte) (*Specification, error)
      Validate(spec *Specification) error
      GetEndpoints() []Endpoint
      GetSchemas() map[string]*Schema
  }
  ```

#### Tarea 1.4.2: Implementación Parser
- **Archivo**: `pkg/openapi/parser.go`
- **Struct**: `OpenAPIParser` implementando `SpecParser`
- **Usar**: `github.com/getkin/kin-openapi`
- **Soporte**: OpenAPI 3.0.x y Swagger 2.0
- **Función**: `NewParser() SpecParser`

#### Tarea 1.4.3: Estructuras de Datos
- **Archivo**: `pkg/openapi/schema.go`
- **Structs necesarias**:
  ```go
  type Specification struct {
      Version   string
      Info      InfoObject
      Paths     map[string]PathItem
      Schemas   map[string]*Schema
      Security  []SecurityRequirement
  }
  
  type Endpoint struct {
      Path       string
      Method     string
      OperationID string
      Parameters []Parameter
      Responses  map[string]Response
  }
  ```

#### Tarea 1.4.4: Tests Unitarios
- **Archivo**: `pkg/openapi/parser_test.go`
- **Test cases**:
  - Parse valid OpenAPI 3.0 spec
  - Parse valid Swagger 2.0 spec
  - Parse invalid spec (should fail)
  - Extract endpoints correctly
  - Extract schemas correctly
- **Fixtures**: Crear specs de ejemplo en `test/fixtures/`

## FASE 2: Servidor HTTP Core (Semanas 3-4)

### 2.1 Servidor HTTP Base

#### Tarea 2.1.1: Servidor FastHTTP
- **Archivo**: `pkg/api/server.go`
- **Struct principal**:
  ```go
  type Server struct {
      config    *config.ServerConfig
      router    *Router
      server    *fasthttp.Server
      logger    *zap.Logger
      metrics   *metrics.Collector
  }
  ```
- **Funciones**:
  - `NewServer(cfg *config.Config) *Server`
  - `Start() error`
  - `Stop() error`
  - `Shutdown(ctx context.Context) error`

#### Tarea 2.1.2: Router Dinámico
- **Archivo**: `pkg/api/router.go`
- **Struct**:
  ```go
  type Router struct {
      routes    map[string]map[string]HandlerFunc
      spec      *openapi.Specification
      generator *openapi.DataGenerator
  }
  ```
- **Funciones**:
  - `RegisterEndpoint(method, path string, handler HandlerFunc)`
  - `Match(method, path string) (HandlerFunc, map[string]string, bool)`
  - `LoadFromSpec(spec *openapi.Specification) error`

#### Tarea 2.1.3: Middleware Stack
- **Archivo**: `pkg/api/middleware.go`
- **Middlewares básicos**:
  - Logger middleware (request/response logging)
  - Metrics middleware (prometheus metrics)
  - Recovery middleware (panic recovery)
  - CORS middleware (configurable CORS)
  - Timeout middleware (request timeout)
- **Función**: `BuildMiddlewareStack(cfg *config.Config) []Middleware`

#### Tarea 2.1.4: Handlers Base
- **Archivo**: `pkg/api/handlers.go`
- **Handler principal**: `MockHandler(ctx *fasthttp.RequestCtx)`
- **Funciones**:
  - Extraer endpoint de request
  - Generar mock response
  - Aplicar chaos scenarios
  - Log request/response
- **Error handling**: 404 para endpoints no encontrados

### 2.2 Generación de Mock Data

#### Tarea 2.2.1: Data Generator Core
- **Archivo**: `pkg/openapi/generator.go`
- **Interface**:
  ```go
  type DataGenerator interface {
      Generate(schema *Schema, context *GenerationContext) (interface{}, error)
      RegisterCustomGenerator(name string, generator CustomGenerator)
      SetLocale(locale string)
  }
  ```
- **Implementación**: `DefaultDataGenerator` struct
- **Integrar**: `github.com/brianvoe/gofakeit/v6`

#### Tarea 2.2.2: Generadores por Tipo
- **Archivo**: `pkg/openapi/generators.go`
- **Generadores específicos**:
  - `generateString(schema *Schema) string`
  - `generateInteger(schema *Schema) int64`
  - `generateNumber(schema *Schema) float64`
  - `generateBoolean() bool`
  - `generateArray(schema *Schema, ctx *GenerationContext) []interface{}`
  - `generateObject(schema *Schema, ctx *GenerationContext) map[string]interface{}`

#### Tarea 2.2.3: Formatos Específicos
- **Archivo**: `pkg/openapi/formats.go`
- **Soporte para formatos**:
  - `email`: Generar emails válidos
  - `uri`: Generar URIs válidos
  - `date`: Formato ISO date
  - `date-time`: Formato RFC3339
  - `uuid`: UUIDs v4
  - `password`: Strings seguros
- **Función**: `GenerateByFormat(format string, schema *Schema) interface{}`

#### Tarea 2.2.4: Contexto de Generación
- **Archivo**: `pkg/openapi/context.go`
- **Struct**:
  ```go
  type GenerationContext struct {
      MaxDepth     int
      CurrentDepth int
      Visited      map[string]bool
      Locale       string
      Faker        *gofakeit.Faker
  }
  ```
- **Prevenir**: Referencias circulares infinitas
- **Configurar**: Profundidad máxima, locale, semilla random

### 2.3 Sistema de Templates

#### Tarea 2.3.1: Template Engine
- **Archivo**: `pkg/templates/engine.go`
- **Usar**: Go template/text con funciones personalizadas
- **Struct**:
  ```go
  type TemplateEngine struct {
      templates map[string]*template.Template
      functions template.FuncMap
  }
  ```
- **Funciones**:
  - `RegisterTemplate(name string, tmpl string) error`
  - `Execute(name string, data interface{}) (string, error)`
  - `RegisterFunction(name string, fn interface{})`

#### Tarea 2.3.2: Funciones de Template
- **Archivo**: `pkg/templates/functions.go`
- **Funciones disponibles**:
  - `fake`: Acceso a gofakeit (`{{ fake "name.first" }}`)
  - `random`: Números random (`{{ random 1 100 }}`)
  - `uuid`: Generar UUID (`{{ uuid }}`)
  - `now`: Fecha actual (`{{ now "2006-01-02" }}`)
  - `env`: Variables de entorno (`{{ env "API_KEY" }}`)
  - `json`: Serializar a JSON (`{{ json . }}`)
- **Registrar**: Todas las funciones en `template.FuncMap`

#### Tarea 2.3.3: Response Templates
- **Soporte**: Templates en responses de OpenAPI spec
- **Ejemplo uso**:
  ```yaml
  responses:
    200:
      description: User list
      content:
        application/json:
          example: |
            {
              "users": [
                {{ range $i := until 5 }}
                {
                  "id": {{ random 1 1000 }},
                  "name": "{{ fake "name.name" }}",
                  "email": "{{ fake "internet.email" }}"
                }{{ if not (last $i) }},{{ end }}
                {{ end }}
              ]
            }
  ```

#### Tarea 2.3.4: Template Validation
- **Archivo**: `pkg/templates/validator.go`
- **Función**: `ValidateTemplate(tmpl string) error`
- **Validar**: Sintaxis de template, funciones existentes
- **Error handling**: Errores descriptivos de parsing

### 2.4 Hot Reload System

#### Tarea 2.4.1: File Watcher
- **Archivo**: `internal/hotreload/watcher.go`
- **Usar**: `github.com/fsnotify/fsnotify`
- **Struct**:
  ```go
  type FileWatcher struct {
      watcher   *fsnotify.Watcher
      files     []string
      callback  func(string) error
      logger    *zap.Logger
  }
  ```
- **Watch**: Archivos OpenAPI spec y configuración

#### Tarea 2.4.2: Reload Logic
- **Archivo**: `internal/hotreload/reloader.go`
- **Función**: `ReloadServer(specPath string) error`
- **Proceso**:
  1. Validar nuevo spec
  2. Crear nuevo router
  3. Atomic swap del router actual
  4. Log del reload exitoso
- **Rollback**: En caso de spec inválido

#### Tarea 2.4.3: Integración con Server
- **Modificar**: `pkg/api/server.go`
- **Agregar**: Campo `reloader *hotreload.Reloader`
- **Función**: `EnableHotReload(specPath string) error`
- **Callback**: Actualizar router cuando cambie spec

## FASE 3: Funcionalidades Avanzadas (Semanas 5-6)

### 3.1 Motor de Chaos Testing

#### Tarea 3.1.1: Chaos Engine Core
- **Archivo**: `pkg/chaos/engine.go`
- **Interface**:
  ```go
  type ChaosEngine interface {
      LoadScenarios(scenarios []ScenarioConfig) error
      ShouldApplyChaos(endpoint string) (bool, ChaosAction)
      ApplyChaos(action ChaosAction, ctx *fasthttp.RequestCtx) error
  }
  ```
- **Implementación**: `DefaultChaosEngine` struct
- **Configuración**: Probability-based chaos injection

#### Tarea 3.1.2: Latency Injection
- **Archivo**: `pkg/chaos/latency.go`
- **Struct**:
  ```go
  type LatencyInjector struct {
      MinDelay    time.Duration
      MaxDelay    time.Duration
      Probability float64
      Endpoints   []string
  }
  ```
- **Función**: `InjectLatency(ctx *fasthttp.RequestCtx) error`
- **Implementar**: Sleep random entre min/max delay

#### Tarea 3.1.3: Error Injection
- **Archivo**: `pkg/chaos/faults.go`
- **Struct**:
  ```go
  type ErrorInjector struct {
      ErrorCodes  []int
      Probability float64
      Endpoints   []string
      CustomBody  string
  }
  ```
- **Función**: `InjectError(ctx *fasthttp.RequestCtx) error`
- **Return**: HTTP error codes (500, 502, 503, etc.)

#### Tarea 3.1.4: Configuración Chaos
- **Archivo**: `pkg/chaos/config.go`
- **Struct**:
  ```go
  type ChaosConfig struct {
      Enabled   bool             `yaml:"enabled"`
      Scenarios []ScenarioConfig `yaml:"scenarios"`
  }
  
  type ScenarioConfig struct {
      Name        string    `yaml:"name"`
      Type        string    `yaml:"type"`  // latency, error, timeout
      Endpoints   []string  `yaml:"endpoints"`
      Probability float64   `yaml:"probability"`
      Parameters  map[string]interface{} `yaml:"parameters"`
  }
  ```

### 3.2 Recording y Replay System

#### Tarea 3.2.1: Request Recorder
- **Archivo**: `pkg/recorder/recorder.go`
- **Struct**:
  ```go
  type Recorder struct {
      storage   Storage
      filters   []RecordingFilter
      enabled   bool
      logger    *zap.Logger
  }
  ```
- **Funciones**:
  - `Record(req *http.Request, resp *http.Response) error`
  - `Start(config RecordingConfig) error`
  - `Stop() error`

#### Tarea 3.2.2: Storage Backend
- **Archivo**: `pkg/recorder/storage.go`
- **Interface**:
  ```go
  type Storage interface {
      Save(recording *Recording) error
      Load(id string) (*Recording, error)
      List() ([]*Recording, error)
      Delete(id string) error
  }
  ```
- **Implementar**: File-based storage como default
- **Formato**: JSON lines para recordings

#### Tarea 3.2.3: Traffic Replay
- **Archivo**: `pkg/recorder/replay.go`
- **Struct**:
  ```go
  type Replayer struct {
      recordings []*Recording
      server     *http.Server
      logger     *zap.Logger
  }
  ```
- **Función**: `ReplayTraffic(recordings []*Recording) error`
- **Features**: Exact replay, parameterized replay

#### Tarea 3.2.4: Recording Format
- **Archivo**: `pkg/recorder/types.go`
- **Struct**:
  ```go
  type Recording struct {
      ID        string            `json:"id"`
      Timestamp time.Time         `json:"timestamp"`
      Request   RecordedRequest   `json:"request"`
      Response  RecordedResponse  `json:"response"`
      Metadata  map[string]string `json:"metadata"`
  }
  ```

### 3.3 Plugin Architecture

#### Tarea 3.3.1: Plugin Interfaces
- **Archivo**: `pkg/plugins/interface.go`
- **Interfaces**:
  ```go
  type Plugin interface {
      Name() string
      Version() string
      Description() string
      Init(config map[string]interface{}) error
      Process(ctx context.Context, req *Request) (*Response, error)
      Cleanup() error
  }
  
  type Middleware interface {
      Plugin
      PreProcess(ctx context.Context, req *Request) error
      PostProcess(ctx context.Context, resp *Response) error
  }
  ```

#### Tarea 3.3.2: Plugin Manager
- **Archivo**: `pkg/plugins/manager.go`
- **Struct**:
  ```go
  type Manager struct {
      plugins   map[string]Plugin
      loader    *Loader
      registry  *Registry
      logger    *zap.Logger
  }
  ```
- **Funciones**:
  - `LoadPlugin(name string, config map[string]interface{}) error`
  - `UnloadPlugin(name string) error`
  - `ListPlugins() []PluginInfo`

#### Tarea 3.3.3: Built-in Plugins
- **Archivo**: `pkg/plugins/builtin.go`
- **Plugins a implementar**:
  - `AuthPlugin`: JWT/API key validation
  - `RateLimitPlugin`: Request rate limiting
  - `CORSPlugin`: CORS headers management
  - `LoggingPlugin`: Request/response logging
- **Cada plugin**: Implementar interfaces definidas

#### Tarea 3.3.4: Plugin Configuration
- **Archivo**: `pkg/plugins/config.go`
- **Struct**:
  ```go
  type PluginConfig struct {
      Name    string                 `yaml:"name"`
      Enabled bool                   `yaml:"enabled"`
      Config  map[string]interface{} `yaml:"config"`
  }
  ```
- **Load**: Plugins desde configuración YAML

## FASE 4: Experiencia de Desarrollador (Semanas 7-8)

### 4.1 Terminal UI Interactiva

#### Tarea 4.1.1: TUI Framework
- **Archivo**: `pkg/cli/tui.go`
- **Usar**: `github.com/charmbracelet/bubbletea`
- **Modelo principal**:
  ```go
  type TUIModel struct {
      tabs        []Tab
      activeTab   int
      server      *api.Server
      metrics     *metrics.Collector
      logs        []LogEntry
      quit        bool
  }
  ```

#### Tarea 4.1.2: Dashboard de Métricas
- **Tab**: "Metrics" en TUI
- **Mostrar en tiempo real**:
  - Requests per second
  - Response time percentiles
  - Error rate
  - Active connections
  - Memory usage
- **Update**: Cada 1 segundo con datos de metrics collector

#### Tarea 4.1.3: Log Viewer
- **Tab**: "Logs" en TUI
- **Features**:
  - Colored log levels
  - Log filtering por level/component
  - Scroll through log history
  - Real-time log streaming
- **Buffer**: Últimos 1000 log entries

#### Tarea 4.1.4: Configuration Editor
- **Tab**: "Config" en TUI
- **Features**:
  - Edit configuration interactivamente
  - Validate changes before applying
  - Hot reload configuration
  - Reset to defaults
- **Form fields**: Para cada configuración importante

### 4.2 Enhanced CLI Commands

#### Tarea 4.2.1: Load Testing Command
- **Archivo**: `cmd/mocker/loadtest.go`
- **Comando**: `mocker load-test`
- **Flags**:
  - `--rps` (int): Requests per second
  - `--duration` (duration): Test duration
  - `--concurrency` (int): Concurrent clients
  - `--endpoint` (string): Specific endpoint to test
- **Output**: Real-time metrics durante test

#### Tarea 4.2.2: Chaos Command
- **Archivo**: `cmd/mocker/chaos.go`
- **Comando**: `mocker chaos`
- **Subcomandos**:
  - `chaos start --scenario <name>`: Start chaos scenario
  - `chaos stop`: Stop all chaos scenarios
  - `chaos list`: List available scenarios
- **Interactive mode**: Para configurar scenarios

#### Tarea 4.2.3: Record/Replay Commands
- **Archivo**: `cmd/mocker/record.go`
- **Comandos**:
  - `mocker record --target <url>`: Start recording
  - `mocker replay <recording-file>`: Replay traffic
  - `mocker recordings list`: List all recordings
  - `mocker recordings delete <id>`: Delete recording

#### Tarea 4.2.4: Shell Completion
- **Archivo**: `pkg/cli/completion.go`
- **Generate**: Bash, Zsh, Fish completions
- **Commands**: `mocker completion bash|zsh|fish`
- **Features**: Complete flags, file paths, scenarios

### 4.3 CI/CD Integration

#### Tarea 4.3.1: Docker Support
- **Archivo**: `Dockerfile`
- **Multi-stage build**: Builder y runtime stages
- **Base image**: Alpine Linux para tamaño mínimo
- **Expose**: Puerto 8080
- **Healthcheck**: Endpoint `/health`
- **Example**: Docker compose con configuración

#### Tarea 4.3.2: Kubernetes Manifests
- **Directorio**: `examples/k8s/`
- **Archivos**:
  - `deployment.yaml`: Deployment con replicas
  - `service.yaml`: Service para exposición
  - `configmap.yaml`: Configuración externa
  - `ingress.yaml`: Ingress para routing
- **Health probes**: Readiness y liveness

#### Tarea 4.3.3: GitHub Actions
- **Archivo**: `examples/ci/github-actions.yaml`
- **Workflow steps**:
  - Start mocker in background
  - Wait for readiness
  - Run integration tests
  - Chaos testing
  - Collect metrics
- **Matrix**: Multiple versions de OpenAPI specs

#### Tarea 4.3.4: Background Daemon Mode
- **Comando**: `mocker daemon`
- **Subcomandos**:
  - `daemon start`: Start in background
  - `daemon stop`: Stop daemon
  - `daemon status`: Check daemon status
  - `daemon logs`: View daemon logs
- **PID file**: Para daemon management

## FASE 5: Optimización y Distribución (Semanas 9-10)

### 5.1 Performance y Monitoring

#### Tarea 5.1.1: Metrics Collection
- **Archivo**: `internal/metrics/collector.go`
- **Metrics a trackear**:
  - HTTP request count (by method, path, status)
  - Request duration histogram
  - Response size histogram
  - Active connections gauge
  - Memory usage gauge
  - Go runtime metrics
- **Store**: In-memory con sliding window

#### Tarea 5.1.2: Prometheus Integration
- **Archivo**: `internal/metrics/prometheus.go`
- **Endpoint**: `/metrics` para Prometheus scraping
- **Metrics format**: Prometheus format standard
- **Labels**: method, path, status_code, chaos_scenario
- **Custom metrics**: Plugin-specific metrics

#### Tarea 5.1.3: Built-in Dashboard
- **Archivo**: `internal/metrics/dashboard.go`
- **Endpoint**: `/dashboard` con HTML dashboard
- **Features**:
  - Real-time charts (usando Chart.js)
  - Request logs table
  - Configuration display
  - Health status
- **WebSocket**: Para updates en tiempo real

#### Tarea 5.1.4: Memory Caching
- **Archivo**: `internal/cache/memory.go`
- **Features**:
  - LRU eviction policy
  - Configurable max size
  - TTL support para entries
  - Thread-safe access
- **Cache**: Parsed specs, generated responses

### 5.2 Build y Release Process

#### Tarea 5.2.1: GoReleaser Configuration
- **Archivo**: `.goreleaser.yml`
- **Platforms**: linux, darwin, windows
- **Architectures**: amd64, arm64
- **Features**:
  - Binary compression
  - Checksums generation
  - Docker image builds
  - Package manager integration
- **Build flags**: Version, commit, build time

#### Tarea 5.2.2: Build Scripts
- **Archivo**: `scripts/build.sh`
- **Functions**:
  - Cross-compilation para todas las platforms
  - Version embedding
  - Binary signing (si disponible)
  - Archive creation
- **Uso**: `make build` ejecuta el script

#### Tarea 5.2.3: Package Manager Integration
- **Homebrew**: Formula en tap dedicado
- **Scoop**: Windows package manager
- **AUR**: Arch User Repository
- **apt/yum**: Debian/RedHat packages
- **Docker Hub**: Automated image builds

#### Tarea 5.2.4: Auto-updater
- **Archivo**: `pkg/updater/updater.go`
- **Features**:
  - Check for updates en startup (configurable)
  - Download y verify checksums
  - Replace binary atomically
  - Rollback en caso de error
- **Command**: `mocker update` para manual updates

### 5.3 Documentation y Examples

#### Tarea 5.3.1: API Documentation
- **Archivo**: `docs/api.md`
- **Contenido**:
  - All CLI commands con examples
  - Configuration file reference
  - Plugin development guide
  - REST API endpoints (metrics, dashboard)
- **Format**: Markdown con code examples

#### Tarea 5.3.2: Configuration Guide
- **Archivo**: `docs/configuration.md`
- **Secciones**:
  - Server configuration options
  - Chaos scenarios configuration
  - Plugin configuration
  - Environment variables
  - Performance tuning tips

#### Tarea 5.3.3: Example OpenAPI Specs
- **Directorio**: `test/examples/`
- **Specs de ejemplo**:
  - `petstore.yaml`: Classic Swagger petstore
  - `banking.yaml`: Banking API con auth
  - `ecommerce.yaml`: E-commerce con complex schemas
  - `microservices.yaml`: Multiple services merged
- **Cada spec**: Incluir README con uso recomendado

#### Tarea 5.3.4: Plugin Development Guide
- **Archivo**: `docs/plugins.md`
- **Content**:
  - Plugin interface implementation
  - Build y distribution
  - Configuration handling
  - Testing strategies
  - Example plugin walkthrough

## CRITERIOS DE ACEPTACIÓN Y VALIDACIÓN

### Para cada tarea:
1. **Código compilar sin errores**: `go build ./...`
2. **Tests passing**: `go test ./...` (cobertura > 80%)
3. **Linting clean**: `golangci-lint run`
4. **Documentación actualizada**: Comentarios GoDoc
5. **Example usage**: Cada feature con ejemplo funcional

### Benchmarks mínimos:
- **Throughput**: > 10,000 RPS en hardware estándar
- **Latency**: P99 < 5ms para responses simples
- **Memory**: < 50MB bajo carga normal
- **Startup time**: < 2 segundos cold start

### Integration tests:
- **OpenAPI specs**: Parsing de specs reales (Stripe, GitHub, etc.)
- **Load testing**: Sostener 1000 RPS por 5 minutos
- **Chaos testing**: Todos los scenarios funcionando
- **Hot reload**: Sin drops de requests durante reload

Este plan está diseñado para que Claude Code pueda ejecutar cada tarea específica usando las herramientas disponibles (Write, Edit, Bash, etc.) con instrucciones claras y verificables.