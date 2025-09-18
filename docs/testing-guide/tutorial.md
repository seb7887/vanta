# Vanta CLI - Tutorial Completo de Testing

Este tutorial te guiará paso a paso para testear todas las funcionalidades del CLI de Vanta usando la especificación OpenAPI comprehensiva incluida.

## Índice

- [Configuración Inicial](#configuración-inicial)
- [1. Command: start](#1-command-start)
- [2. Command: config](#2-command-config)
- [3. Command: validate](#3-command-validate)
- [4. Command: record](#4-command-record)
- [5. Command: chaos](#5-command-chaos)
- [6. Command: state](#6-command-state)
- [7. Command: tui](#7-command-tui)
- [8. Command: version](#8-command-version)
- [Testing Avanzado](#testing-avanzado)
- [Troubleshooting](#troubleshooting)

## Configuración Inicial

### Prerequisitos

1. **Instalar Vanta CLI**: Asegúrate de tener el binario `vanta` compilado y en tu PATH
2. **Archivos de test**: Usa los archivos incluidos en este repositorio
3. **Tools adicionales**: curl, jq (opcional para parsing JSON), httpie (opcional)

### Archivos de Testing

```bash
# Navegue al directorio del proyecto
cd /path/to/vanta

# Verifique que los archivos de testing estén disponibles
ls spec/vanta-test-api.yaml          # Especificación OpenAPI comprehensiva
ls examples/                         # Configuraciones de ejemplo
ls vanta.yaml                       # Configuración base
```

### Verificación Básica

```bash
# Verificar que Vanta CLI esté instalado correctamente
vanta version

# Salida esperada:
# Version: v1.0.0
# Commit: abc1234
# Build Time: 2025-01-15T10:00:00Z
```

---

## 1. Command: start

El comando `start` es el comando principal para iniciar el servidor mock.

### 1.1 Inicio Básico

```bash
# Iniciar servidor con la especificación de testing
vanta start spec/vanta-test-api.yaml

# Salida esperada:
# INFO: Starting vanta server spec=spec/vanta-test-api.yaml port=8080 host=0.0.0.0
# INFO: Server created successfully, starting...
# INFO: Server running on http://0.0.0.0:8080
```

**Comportamiento esperado:**
- El servidor debe iniciarse en el puerto 8080
- Debe cargar la especificación OpenAPI sin errores
- Los endpoints deben estar disponibles según la spec

### 1.2 Verificar Endpoints Básicos

En otra terminal, mientras el servidor corre:

```bash
# Test health endpoint
curl http://localhost:8080/health

# Salida esperada (JSON):
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:30:00Z",
  "uptime": 3600,
  "version": "1.0.0"
}

# Test metrics endpoint
curl http://localhost:8080/metrics

# Salida esperada (JSON con métricas del servidor):
{
  "requests_total": 1,
  "requests_per_second": 0.1,
  "errors_total": 0,
  "error_rate": 0.0,
  "average_latency_ms": 1.5,
  "p95_latency_ms": 2.1,
  "memory_usage_mb": 25.6,
  "cpu_usage_percent": 0.5,
  "active_connections": 1
}
```

### 1.3 Test con Configuración Personalizada

```bash
# Detener el servidor anterior (Ctrl+C)

# Crear configuración de testing personalizada
cat > test-config.yaml << EOF
server:
  port: 9090
  host: "127.0.0.1"
  read_timeout: 15s
  write_timeout: 15s
mock:
  seed: 42
  locale: "es"
logging:
  level: "debug"
  format: "text"
EOF

# Iniciar con configuración personalizada
vanta start spec/vanta-test-api.yaml --config test-config.yaml --port 9090

# Salida esperada:
# DEBUG: Loading configuration from file file=test-config.yaml
# INFO: Starting vanta server spec=spec/vanta-test-api.yaml port=9090 host=127.0.0.1
# DEBUG: Using Spanish locale for data generation
# INFO: Server running on http://127.0.0.1:9090
```

### 1.4 Test con Parámetros de Línea de Comandos

```bash
# Iniciar con overrides por CLI
vanta start spec/vanta-test-api.yaml \
  --port 8888 \
  --host localhost \
  --config test-config.yaml

# Verificar que los parámetros CLI tienen prioridad
curl http://localhost:8888/health
# Debe responder en puerto 8888, no 9090
```

### 1.5 Test de Datos Generados

```bash
# Test endpoint de usuarios (debe generar datos mock)
curl http://localhost:8080/users

# Salida esperada (array de usuarios generados):
{
  "users": [
    {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com",
      "role": "admin",
      "active": true,
      "created_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "username": "testuser",
      "email": "test@example.com",
      "role": "user",
      "active": true,
      "created_at": "2025-01-02T10:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 2,
    "total_pages": 1
  }
}

# Test endpoint de productos con parámetros
curl "http://localhost:8080/products?category=electronics&min_price=100"

# Debe filtrar productos según parámetros de query
```

---

## 2. Command: config

El comando `config` gestiona archivos de configuración.

### 2.1 Inicializar Configuración

```bash
# Crear nueva configuración
vanta config init --output my-config.yaml

# Salida esperada:
# Configuration file created: my-config.yaml

# Verificar contenido
cat my-config.yaml

# Debe mostrar configuración YAML completa con valores por defecto
```

### 2.2 Validar Configuración

```bash
# Validar configuración existente
vanta config validate vanta.yaml

# Salida esperada:
# Configuration file is valid: vanta.yaml

# Test con configuración inválida
cat > invalid-config.yaml << EOF
server:
  port: "invalid"  # Error: debe ser número
  host: 123        # Error: debe ser string
EOF

vanta config validate invalid-config.yaml

# Salida esperada (con error):
# Error: configuration validation failed: port must be a number
```

### 2.3 Editar Configuración

```bash
# Intentar editar configuración
vanta config edit my-config.yaml

# Salida esperada:
# Opening /path/to/my-config.yaml in vi...
# Note: Interactive editing is not yet implemented in this version.
# Please manually edit the file: /path/to/my-config.yaml
```

---

## 3. Command: validate

El comando `validate` valida especificaciones OpenAPI y genera reportes.

### 3.1 Validar Especificación

```bash
# Validar especificación de testing
vanta validate spec spec/vanta-test-api.yaml

# Salida esperada:
# OpenAPI Specification Validation
# ================================
#
# Status: VALID
# Title: Vanta Test API - Comprehensive Testing Specification
# Version: 1.0.0
# Endpoints: 25
# Schemas: 35

# Test en formato JSON
vanta validate spec spec/vanta-test-api.yaml --format json

# Salida esperada (JSON estructurado):
{
  "valid": true,
  "errors": [],
  "warnings": [],
  "info": {
    "version": "3.0.3",
    "title": "Vanta Test API - Comprehensive Testing Specification",
    "endpoints": 25,
    "schemas": 35
  }
}
```

### 3.2 Validar con Modo Estricto

```bash
# Validar en modo estricto
vanta validate spec spec/vanta-test-api.yaml --strict --examples

# Debe validar también todos los ejemplos en la especificación
```

### 3.3 Generar Reporte de Cobertura

```bash
# Primero, iniciar servidor para generar tráfico
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Generar algo de tráfico para testing de cobertura
curl http://localhost:8080/health
curl http://localhost:8080/users
curl http://localhost:8080/products

# Generar reporte de cobertura
vanta validate coverage spec/vanta-test-api.yaml

# Salida esperada:
# API Coverage Report
# ===================
#
# Overall Coverage: 12.0%
# Covered Endpoints: 3/25
#
# Endpoint Details:
#   GET /health - COVERED (1 requests)
#   GET /users - COVERED (1 requests)
#   GET /products - COVERED (1 requests)
#   POST /users - NOT COVERED (0 requests)
#   ...

# Detener servidor
kill $SERVER_PID
```

### 3.4 Generar Reporte de Compliance

```bash
# Generar reporte de compliance
vanta validate compliance spec/vanta-test-api.yaml --format json

# Salida esperada (JSON):
{
  "compliance_percent": 95.5,
  "total_requests": 100,
  "valid_requests": 95,
  "invalid_requests": 5,
  "violations": [
    {
      "endpoint": "POST /users",
      "method": "POST",
      "type": "validation_error",
      "message": "Missing required field: email",
      "count": 3
    }
  ]
}
```

---

## 4. Command: record

El comando `record` permite grabar y reproducir tráfico HTTP.

### 4.1 Iniciar Grabación

```bash
# Iniciar grabación básica
vanta record start

# Salida esperada:
# 🎬 Starting recording...
# ✅ Recording enabled
# 📁 Storage directory: ./recordings
# 📊 Max recordings: 1000
# 📏 Max body size: 1048576 bytes
# 🔍 Filters: 0 configured
```

### 4.2 Grabación con Filtros

```bash
# Iniciar grabación con filtros específicos
vanta record start \
  --filter "method:GET" \
  --filter "endpoint:/users" \
  --output ./my-recordings \
  --max-recordings 100

# Salida esperada:
# 🎬 Starting recording...
# ✅ Recording enabled
# 📁 Storage directory: ./my-recordings
# 📊 Max recordings: 100
# 📏 Max body size: 1048576 bytes
# 🔍 Filters: 2 configured
```

### 4.3 Generar Tráfico para Grabación

Con la grabación activa, en otra terminal:

```bash
# Generar tráfico variado
curl -X GET http://localhost:8080/users
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"Test123!","role":"user"}'
curl -X GET http://localhost:8080/products?category=electronics
curl -X GET http://localhost:8080/health
```

### 4.4 Listar Grabaciones

```bash
# Listar todas las grabaciones
vanta record list

# Salida esperada:
# 📋 Found 4 recordings:
#
# ID                                       METHOD   URI                              STATUS TIMESTAMP
# --------------------------------------------------------------------------------------------------
# abc123def456...                         GET      /users                           200    2025-01-15 10:30:00
# def456abc789...                         POST     /users                           201    2025-01-15 10:30:05
# ghi789def012...                         GET      /products?category=electronics   200    2025-01-15 10:30:10
# jkl012ghi345...                         GET      /health                          200    2025-01-15 10:30:15

# Listar con filtros
vanta record list --method GET --limit 2

# Listar grabaciones recientes
vanta record list --since 1h
```

### 4.5 Ver Detalles de Grabación

```bash
# Ver detalles de una grabación específica
vanta record show abc123def456...

# Salida esperada:
# 🎬 Recording Details
#
# ID:        abc123def456...
# Timestamp: 2025-01-15 10:30:00
# Duration:  2ms
#
# 📨 Request:
#   Method: GET
#   URI:    /users
#   Headers (3):
#     User-Agent: curl/7.68.0
#     Accept: */*
#     Host: localhost:8080
#   Body:   0 bytes
#
# 📤 Response:
#   Status: 200
#   Headers (2):
#     Content-Type: application/json
#     Content-Length: 1234
#   Body:   1234 bytes
#
# 🏷️  Metadata:
#   Source:    http_server
#   Client IP: 127.0.0.1
```

### 4.6 Detener Grabación

```bash
# Detener grabación activa
vanta record stop

# Salida esperada:
# ⏹️  Stopping recording...
# ✅ Recording stopped
```

### 4.7 Reproducir Grabaciones

```bash
# Iniciar servidor de destino para replay (en otro puerto)
vanta start spec/vanta-test-api.yaml --port 8081 &
REPLAY_SERVER_PID=$!

# Reproducir todas las grabaciones
vanta record replay --target http://localhost:8081

# Salida esperada:
# 🔄 Starting replay to http://localhost:8081...
# 📋 Loaded 4 recordings for replay
#
# 📊 Replay completed:
#    Total requests: 4
#    Successful: 4
#    Failed: 0
#    Average latency: 15ms
#    Duration: 2.5s

# Reproducir grabaciones específicas
vanta record replay \
  --target http://localhost:8081 \
  --ids abc123def456,def456abc789 \
  --concurrency 2 \
  --delay 500ms

# Detener servidor de replay
kill $REPLAY_SERVER_PID
```

### 4.8 Exportar Grabaciones

```bash
# Exportar a formato HAR
vanta record export --format har --output recordings.har

# Exportar a colección Postman
vanta record export --format postman --output collection.json

# Exportar como comandos cURL
vanta record export --format curl --output commands.sh

# Salida esperada:
# 📤 Exporting recordings in curl format...
# 📋 Loaded 4 recordings for export
# Export completed: commands.sh
```

### 4.9 Eliminar Grabaciones

```bash
# Eliminar grabación específica
vanta record delete abc123def456...

# Salida esperada:
# ✅ Deleted recording abc123def456...

# Eliminar todas las grabaciones (con confirmación)
vanta record delete --all

# Salida esperada:
# ⚠️  Are you sure you want to delete ALL recordings? (y/N): y
# ✅ All recordings deleted

# Eliminar sin confirmación
vanta record delete --all --force
```

---

## 5. Command: chaos

El comando `chaos` implementa chaos engineering para testear resiliencia.

### 5.1 Configurar Scenarios de Chaos

Primero, crear configuración de chaos:

```bash
cat > chaos-config.yaml << EOF
server:
  port: 8080
chaos:
  enabled: true
  scenarios:
    - name: "api_latency"
      type: "latency"
      probability: 0.3
      endpoints: ["/users", "/products"]
      parameters:
        min_delay: "100ms"
        max_delay: "2s"

    - name: "random_errors"
      type: "error"
      probability: 0.1
      endpoints: ["/users/*", "/products/*"]
      parameters:
        status_codes: [500, 502, 503]

    - name: "slow_database"
      type: "latency"
      probability: 0.2
      endpoints: ["/analytics/*"]
      parameters:
        min_delay: "1s"
        max_delay: "5s"
logging:
  level: "info"
EOF
```

### 5.2 Listar Scenarios Disponibles

```bash
# Listar scenarios configurados
vanta chaos list --config chaos-config.yaml

# Salida esperada:
# 📋 Available Chaos Scenarios
#
# 🎯 api_latency
#    Type: latency
#    Endpoints: [/users /products]
#    Probability: 30.0%
#    Parameters:
#      max_delay: 2s
#      min_delay: 100ms
#
# 🎯 random_errors
#    Type: error
#    Endpoints: [/users/* /products/*]
#    Probability: 10.0%
#    Parameters:
#      status_codes: [500 502 503]
#
# 🎯 slow_database
#    Type: latency
#    Endpoints: [/analytics/*]
#    Probability: 20.0%
#    Parameters:
#      max_delay: 5s
#      min_delay: 1s
```

### 5.3 Iniciar Chaos Testing

```bash
# Iniciar todos los scenarios
vanta chaos start --config chaos-config.yaml

# Salida esperada:
# ✅ Chaos testing started with 3 scenario(s)
#   - api_latency (latency): 30.0% probability on [/users /products]
#   - random_errors (error): 10.0% probability on [/users/* /products/*]
#   - slow_database (latency): 20.0% probability on [/analytics/*]
# ♾️  Running indefinitely (Ctrl+C to stop)

# En otra terminal, generar tráfico para observar efectos de chaos
for i in {1..20}; do
  echo "Request $i:"
  time curl -s http://localhost:8080/users | jq '.users | length'
  sleep 1
done

# Debería observar:
# - Algunos requests con latencia adicional (100ms-2s)
# - Algunos requests fallando con errores 500/502/503
# - Variación en tiempos de respuesta
```

### 5.4 Iniciar Scenario Específico

```bash
# Iniciar solo scenario de latencia por 5 minutos
vanta chaos start \
  --config chaos-config.yaml \
  --scenario api_latency \
  --duration 5m

# Salida esperada:
# ✅ Chaos testing started with 1 scenario(s)
#   - api_latency (latency): 30.0% probability on [/users /products]
# ⏰ Will run for 5m0s
#
# [después de 5 minutos]
# ⏰ Duration elapsed, stopping chaos testing
#
# 📊 Final Statistics:
#   Total requests: 45
#   Chaos applied: 13
#   Failed injections: 0
#   Chaos rate: 28.89%
# ✅ Chaos testing stopped
```

### 5.5 Verificar Estado de Chaos

```bash
# Ver estado actual de chaos testing
vanta chaos status --config chaos-config.yaml

# Salida esperada:
# 📋 Chaos Testing Status
#
# Configuration file: chaos-config.yaml
# Chaos enabled: true
# Scenarios configured: 3
#
# 📝 Configured Scenarios:
#   1. api_latency (latency)
#      Endpoints: [/users /products]
#      Probability: 30.0%
#      Parameters: map[max_delay:2s min_delay:100ms]
#
#   2. random_errors (error)
#      Endpoints: [/users/* /products/*]
#      Probability: 10.0%
#      Parameters: map[status_codes:[500 502 503]]
#
#   3. slow_database (latency)
#      Endpoints: [/analytics/*]
#      Probability: 20.0%
#      Parameters: map[max_delay:5s min_delay:1s]
```

### 5.6 Parar Chaos Testing

```bash
# Parar chaos testing activo
vanta chaos stop

# Salida esperada:
# 🛑 Chaos testing stop signal sent
# 💡 Note: To stop chaos testing on a running server, restart the server or use configuration hot-reload
```

---

## 6. Command: state

El comando `state` gestiona el estado del servidor mock.

### 6.1 Configurar State Management

```bash
# Iniciar servidor con state habilitado
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Dar tiempo al servidor para inicializar
sleep 2
```

### 6.2 Establecer Valores en State

```bash
# Establecer valor simple
vanta state set user_count 1000

# Salida esperada:
# Successfully set user_count = 1000

# Establecer valor JSON
vanta state set current_user '{"id":1,"name":"Admin","role":"admin"}'

# Salida esperada:
# Successfully set current_user = map[id:1 name:Admin role:admin]

# Establecer valor desde archivo
echo '{"feature_flags":{"new_ui":true,"beta_features":false}}' > features.json
vanta state set app_config --file features.json

# Establecer valor con TTL
vanta state set session_token "abc123xyz" --ttl 1h

# Salida esperada:
# Successfully set session_token = abc123xyz
# with TTL: 1h0m0s

# Establecer valor en scope específico
vanta state set last_login "2025-01-15T10:30:00Z" --scope user:123

# Salida esperada:
# Successfully set last_login = 2025-01-15T10:30:00Z
# in scope: user:123
```

### 6.3 Obtener Valores del State

```bash
# Obtener valor simple
vanta state get user_count

# Salida esperada (formato pretty por defecto):
{
  "key": "user_count",
  "scope": "",
  "value": 1000
}

# Obtener valor en formato raw
vanta state get user_count --format raw

# Salida esperada:
# 1000

# Obtener valor en formato JSON
vanta state get current_user --format json

# Salida esperada:
{
  "id": 1,
  "name": "Admin",
  "role": "admin"
}

# Obtener valor de scope específico
vanta state get last_login --scope user:123

# Obtener valor y guardar en archivo
vanta state get app_config --output config-backup.json
```

### 6.4 Listar Claves del State

```bash
# Listar todas las claves
vanta state list

# Salida esperada:
{
  "keys": [
    "user_count",
    "current_user",
    "app_config",
    "session_token"
  ],
  "count": 4
}

# Listar en formato texto
vanta state list --format text

# Salida esperada:
# user_count
# current_user
# app_config
# session_token

# Listar scopes disponibles
vanta state list --scope

# Salida esperada:
{
  "scopes": [
    "user:123"
  ]
}
```

### 6.5 Eliminar Valores del State

```bash
# Eliminar clave específica
vanta state delete session_token

# Salida esperada:
# Successfully deleted key: session_token

# Eliminar clave de scope específico
vanta state delete last_login --scope user:123

# Salida esperada:
# Successfully deleted key: last_login
# from scope: user:123
```

### 6.6 Limpiar State

```bash
# Limpiar todo el state (con confirmación)
vanta state clear

# Salida esperada:
# This will permanently delete state data. Are you sure? (y/N): y
# Successfully cleared all state

# Limpiar sin confirmación
vanta state clear --yes

# Limpiar scope específico
vanta state clear --scope user:123 --yes

# Salida esperada:
# Successfully cleared scope: user:123
```

### 6.7 Exportar/Importar State

```bash
# Primero, establecer algunos datos de prueba
vanta state set test_data '{"key1":"value1","key2":"value2"}'
vanta state set counter 42
vanta state set enabled true

# Exportar state completo
vanta state export

# Salida esperada:
# State exported to: state_export_2025-01-15_10-30-45.json
# Exported 3 keys

# Exportar a archivo específico
vanta state export --output my-state-backup.json

# Ver contenido del export
cat my-state-backup.json

# Limpiar state para test de import
vanta state clear --yes

# Importar state desde archivo
vanta state import my-state-backup.json

# Salida esperada:
# Successfully imported state from: my-state-backup.json
# Imported 3 keys
# Existing state was replaced

# Importar con merge (conservar estado existente)
vanta state set new_key "new_value"
vanta state import my-state-backup.json --merge

# Salida esperada:
# Successfully imported state from: my-state-backup.json
# Imported 3 keys
# State was merged with existing data

# Detener servidor
kill $SERVER_PID
```

---

## 7. Command: tui

El comando `tui` lanza una interfaz de usuario en terminal interactiva.

### 7.1 Lanzar TUI Básico

```bash
# Lanzar TUI con configuración por defecto
vanta tui

# Salida esperada:
# INFO: Starting TUI mode config=config.yaml spec= readonly=false
# INFO: Launching Terminal UI...
# INFO: TUI Controls:
#   navigation: Tab/Shift+Tab to switch between panels
#   logs: ↑/↓ to scroll, f to filter, c to clear
#   config: ↑/↓ to navigate, Enter to edit, Ctrl+S to save
#   exit: q or Ctrl+C to quit

# [Se abre interfaz TUI interactiva]
```

### 7.2 TUI con Especificación OpenAPI

```bash
# Lanzar TUI con especificación específica
vanta tui --spec spec/vanta-test-api.yaml

# Debería mostrar:
# - Panel de métricas en tiempo real
# - Panel de logs con requests/responses
# - Panel de configuración
# - Panel de estado del servidor
```

### 7.3 TUI en Modo Solo Lectura

```bash
# Lanzar TUI sin edición de configuración
vanta tui --readonly --spec spec/vanta-test-api.yaml

# El panel de configuración debe ser de solo lectura
```

### 7.4 Navegación en TUI

**Controles de TUI a probar:**

1. **Navegación entre paneles:**
   - `Tab` / `Shift+Tab`: Cambiar entre paneles
   - `q` o `Ctrl+C`: Salir

2. **Panel de Logs:**
   - `↑/↓`: Scroll por logs
   - `f`: Filtrar logs
   - `c`: Limpiar logs
   - `Enter`: Ver detalles de log seleccionado

3. **Panel de Métricas:**
   - Debe mostrar en tiempo real:
     - RPS (Requests Per Second)
     - Latencia promedio
     - Códigos de error
     - Uso de memoria
     - Conexiones activas

4. **Panel de Configuración:**
   - `↑/↓`: Navegar por opciones
   - `Enter`: Editar valor (si no es readonly)
   - `Ctrl+S`: Guardar cambios
   - `Esc`: Cancelar edición

### 7.5 Generar Tráfico para TUI

Con TUI activo, en otra terminal:

```bash
# Script para generar tráfico continuo
for i in {1..100}; do
  curl -s http://localhost:8080/users > /dev/null &
  curl -s http://localhost:8080/products > /dev/null &
  curl -s http://localhost:8080/health > /dev/null &

  # Algunos errores para testear métricas
  if [ $((i % 10)) -eq 0 ]; then
    curl -s http://localhost:8080/test/error/500 > /dev/null &
  fi

  sleep 0.1
done

wait
```

**Comportamiento esperado en TUI:**
- Panel de métricas debe actualizar RPS, latencia, errores
- Panel de logs debe mostrar requests entrantes
- Contadores deben incrementar en tiempo real

---

## 8. Command: version

### 8.1 Información de Versión

```bash
# Mostrar información de versión
vanta version

# Salida esperada:
# Version: v1.0.0
# Commit: abc1234def5678
# Build Time: 2025-01-15T10:00:00Z
# Go Version: go1.21.0
# OS/Arch: linux/amd64
```

---

## Testing Avanzado

### Hot Reload Testing

```bash
# Iniciar servidor con hot reload habilitado
vanta start spec/vanta-test-api.yaml &
SERVER_PID=$!

# Modificar la especificación OpenAPI
cp spec/vanta-test-api.yaml spec/vanta-test-api-modified.yaml

# Agregar nuevo endpoint
cat >> spec/vanta-test-api-modified.yaml << EOF
  /test/new:
    get:
      summary: New test endpoint
      operationId: newTest
      responses:
        '200':
          description: New endpoint
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
EOF

# El servidor debería detectar cambios automáticamente
# Verificar nuevo endpoint
curl http://localhost:8080/test/new

kill $SERVER_PID
```

### Plugin Testing

```bash
# Crear configuración con plugins
cat > plugin-config.yaml << EOF
server:
  port: 8080
plugins:
  - name: "request_logger"
    type: "builtin"
    enabled: true
    config:
      log_level: "info"
      include_body: true

  - name: "rate_limiter"
    type: "builtin"
    enabled: true
    config:
      requests_per_minute: 60
      burst: 10

  - name: "cors_handler"
    type: "builtin"
    enabled: true
    config:
      allowed_origins: ["*"]
      allowed_methods: ["GET", "POST", "PUT", "DELETE"]
EOF

# Iniciar con plugins
vanta start spec/vanta-test-api.yaml --config plugin-config.yaml

# Test CORS headers
curl -H "Origin: https://example.com" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -X OPTIONS \
     http://localhost:8080/users

# Debería incluir headers CORS en respuesta

# Test rate limiting
for i in {1..70}; do
  curl -s http://localhost:8080/health
done

# Después de 60 requests, debería devolver 429 Too Many Requests
```

### Webhook Testing

```bash
# Configurar webhook de prueba con ngrok o servidor local
python3 -m http.server 9999 &
WEBHOOK_SERVER_PID=$!

# Crear webhook en la API
curl -X POST http://localhost:8080/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "url": "http://localhost:9999/webhook",
    "events": ["user.created", "order.created"],
    "secret": "mysecretkey123"
  }'

# Crear usuario para disparar webhook
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "webhooktest",
    "email": "webhook@test.com",
    "password": "Test123!",
    "role": "user"
  }'

# Verificar que webhook server recibió la notificación
kill $WEBHOOK_SERVER_PID
```

### Performance Testing

```bash
# Usar herramientas como ab, wrk, o hey para performance testing

# Test con Apache Bench
ab -n 1000 -c 10 http://localhost:8080/health

# Test con hey (si está instalado)
hey -n 1000 -c 10 http://localhost:8080/users

# Test con curl en paralelo
seq 1 100 | xargs -n1 -P10 -I{} curl -s http://localhost:8080/products > /dev/null

# Verificar métricas durante testing
curl http://localhost:8080/metrics
```

### Data Generation Testing

```bash
# Test diferentes seeds para generación de datos
vanta start spec/vanta-test-api.yaml --config <(cat << EOF
mock:
  seed: 42
  locale: "en"
  max_depth: 3
  default_array_size: 5
  prefer_examples: true
EOF
) &

# Obtener datos con seed 42
curl http://localhost:8080/users > users_seed_42.json

# Reiniciar con seed diferente
pkill vanta
vanta start spec/vanta-test-api.yaml --config <(cat << EOF
mock:
  seed: 99
  locale: "es"
  max_depth: 3
  default_array_size: 5
  prefer_examples: true
EOF
) &

# Obtener datos con seed 99
curl http://localhost:8080/users > users_seed_99.json

# Los datos deben ser diferentes pero consistentes por seed
diff users_seed_42.json users_seed_99.json
```

---

## Troubleshooting

### Problemas Comunes

#### 1. Puerto en Uso

```bash
# Error: "bind: address already in use"
# Solución: Verificar qué proceso usa el puerto
lsof -i :8080
kill <PID>

# O usar puerto diferente
vanta start spec/vanta-test-api.yaml --port 8081
```

#### 2. Especificación OpenAPI Inválida

```bash
# Error: "failed to parse OpenAPI spec"
# Solución: Validar especificación primero
vanta validate spec spec/vanta-test-api.yaml

# Verificar sintaxis YAML
yamllint spec/vanta-test-api.yaml
```

#### 3. Configuración Inválida

```bash
# Error: "configuration validation failed"
# Solución: Validar configuración
vanta config validate my-config.yaml

# Crear configuración limpia
vanta config init --output clean-config.yaml
```

#### 4. Permisos de Archivo

```bash
# Error: "permission denied"
# Solución: Verificar permisos
ls -la spec/vanta-test-api.yaml
chmod 644 spec/vanta-test-api.yaml
```

#### 5. Estado Corrupto

```bash
# Error en state management
# Solución: Limpiar estado
vanta state clear --yes

# O reiniciar completamente
rm -rf ./recordings ./state
```

### Logging y Debug

```bash
# Iniciar con logging debug
vanta start spec/vanta-test-api.yaml --config <(cat << EOF
logging:
  level: "debug"
  format: "text"
  output: "stdout"
  add_caller: true
EOF
)

# Verificar logs del sistema
journalctl -f | grep vanta

# Verificar recursos del sistema
htop
ps aux | grep vanta
```

### Verificación de Health

```bash
# Script para verificar estado del servidor
check_health() {
  response=$(curl -s -w "%{http_code}" http://localhost:8080/health)
  http_code="${response: -3}"

  if [ "$http_code" = "200" ]; then
    echo "✅ Server is healthy"
    return 0
  else
    echo "❌ Server health check failed (HTTP $http_code)"
    return 1
  fi
}

# Verificar salud cada 5 segundos
while true; do
  check_health
  sleep 5
done
```

---

## Resumen de Comandos

| Comando | Funcionalidad | Ejemplo Básico |
|---------|---------------|----------------|
| `start` | Iniciar servidor mock | `vanta start spec/vanta-test-api.yaml` |
| `config` | Gestionar configuración | `vanta config init` |
| `validate` | Validar specs y compliance | `vanta validate spec spec/vanta-test-api.yaml` |
| `record` | Grabar/reproducir tráfico | `vanta record start` |
| `chaos` | Chaos engineering | `vanta chaos start --config chaos.yaml` |
| `state` | Gestionar estado | `vanta state set key value` |
| `tui` | Interfaz terminal | `vanta tui --spec spec/vanta-test-api.yaml` |
| `version` | Info de versión | `vanta version` |

---

## Siguientes Pasos

1. **Testing automatizado**: Crear scripts para automatizar estos tests
2. **CI/CD Integration**: Integrar tests en pipeline de CI/CD
3. **Monitoring**: Configurar monitoring en producción
4. **Custom Plugins**: Desarrollar plugins personalizados
5. **Load Testing**: Realizar tests de carga más exhaustivos

Para testing más específico, consulta el archivo `test-scenarios.md` que contiene casos de uso específicos y tests de regresión.