# SRE Network Discovery v2.0

Herramienta de grado SRE para **descubrimiento de infraestructura de red (CDP/LLDP)** y **validación de puntos de conexión de red** en Windows.

Compilada de forma nativa en **Go**: ejecutable único, ultraligero (~5 MB), sin dependencias externas, sin intérpretes en `%TEMP%` y libre de falsos positivos en Windows Defender / EDR.

---

## Características Principales

- **Captura Nativa L2 (CDP / LLDP)**: Captura anuncios de switches Cisco y multivendor (Cisco Discovery Protocol y Link Layer Discovery Protocol) mediante el subsistema del kernel **ETW (Event Tracing for Windows)** sin necesidad de Wireshark ni WinPcap.
- **Validación de Punto de Conexión de Red**:
  - Estado del enlace físico (Carrier Up/Down).
  - Inspección de direccionamiento IP asignado (IPv4 / IPv6).
  - Detección y sondeo de puerta de enlace (Gateway).
  - Prueba de resolución DNS y canal seguro TLS.
- **Modo Extracción Automática (`--auto`)**:
  - Monitoriza en tiempo real la conexión del cable de red.
  - Al conectarse a una toma o switch, valida el canal seguro, ejecuta la extracción del switch/puerto y lo añade al inventario.
  - Ideal para técnicos recorriendo racks o rosetas de red.
- **Modo Acumulativo (`-a` / `--append`)**:
  - Permite ir sumando escaneos sucesivos al inventario principal sin perder el historial.
  - Deduplica automáticamente por Switch y Puerto, actualizando última hora vista y contador de detecciones.
- **Auditoría Clara y Modular**:
  - Código desacoplado en paquetes limpios y auditables (`pkg/link`, `pkg/protocol`, `pkg/capture`, `pkg/engine`, `pkg/report`).
  - Sin anotaciones redundantes ni dependencias infladas.
- **Exportación Múltiple**:
  - JSON estructurado (`sre_network_inventory.json`).
  - Reporte en Markdown formateado con tablas (`sre_network_inventory.md`).
  - Hoja de cálculo CSV (`sre_network_inventory.csv`).
  - Soporte de envío cifrado HTTPS (`--remote https://...`).

---

## Estructura Modular

```
sre-network-discovery/
├── bin/
│   └── sre-network-discovery.exe    # Binario PE32+ portable para Windows
├── cmd/
│   └── sre-discovery/
│       └── main.go                  # CLI principal y gestión de señales
├── pkg/
│   ├── capture/                     # Motor ETW y recolección de tramas L2
│   ├── engine/                      # Orquestador de escaneo y modo automático
│   ├── link/                        # Validación de punto de red y canal seguro
│   ├── protocol/                    # Decodificador binario de tramas CDP y LLDP
│   └── report/                      # Gestor de inventario acumulativo y exportadores
├── go.mod
└── README.md
```

---

## Uso del Ejecutable en Windows

> **Nota:** La captura de tramas de capa 2 mediante ETW requiere ejecutarse con privilegios de **Administrador**.

### 1. Escaneo Simple (Predeterminado)
Valida el punto de red y captura anuncios durante 62 segundos:
```cmd
sre-network-discovery.exe
```

### 2. Modo Acumulativo (Sumar escaneos a un reporte maestro)
Si estás probando varias bocas de red o switches y quieres un reporte global consolidado:
```cmd
sre-network-discovery.exe -a
```

### 3. Modo Extracción Automática
Deja el programa corriendo mientras conectas cables en distintos puertos:
```cmd
sre-network-discovery.exe --auto -a
```

### 4. Listar adaptadores disponibles
```cmd
sre-network-discovery.exe --list
```

### 5. Parámetros Disponibles

| Parámetro | Descripción | Valor por defecto |
| :--- | :--- | :--- |
| `-i <nombre>` | Nombre específico del adaptador de red | Auto-detectar Ethernet físico |
| `-d <seg>` | Duración del escaneo en segundos | `62` |
| `-v` | Validar conexión de red antes de escanear | `true` |
| `-a` | Modo acumulativo (suma al reporte existente) | `false` |
| `--auto` | Modo daemon (extrae automáticamente al conectar cable) | `false` |
| `-o <archivo>` | Ruta del archivo de salida JSON | `sre_network_inventory.json` |
| `--md` | Generar reporte en Markdown | `true` |
| `--csv` | Generar reporte en CSV | `true` |
| `--remote <url>` | URL HTTPS de canal seguro para envío automático | `""` |
| `--list` | Muestra adaptadores de red elegibles y finaliza | `false` |

---

## Compilación desde Código Fuente

### En Linux (Compilación Cruzada para Windows):
```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/sre-network-discovery.exe ./cmd/sre-discovery
```

### En Windows:
```cmd
go build -ldflags="-s -w" -o sre-network-discovery.exe ./cmd/sre-discovery
```

---

## Pruebas Automatizadas

```bash
go test -v ./...
```
