## SRE Network Discovery v2.1.0

Esta versión representa una evolución integral de la herramienta, introduciendo auditoría **Dual-VLAN (Datos + Voz/VoIP)** para telefonía IP, **EDR Hardening** para ejecución 100% silenciosa e invisible en entornos corporativos de Windows, resiliencia a cambios de cable en caliente (**Hot-Plug**), y un rediseño de interfaz de escritorio de grado profesional (**Liquid Glass Obsidiana**).

---

### 🚀 Novedades y Mejoras Principales

#### 1. 📞 Extracción Dual-VLAN y Soporte para Teléfonos IP (Daisy-Chain)
- **Extracción de VLAN de Voz (VoIP):**
  - **Cisco CDP:** Decodificación binaria nativa de TLV `0x000E` (*Voice VLAN Query*) y TLV `0x000C` (*VoIP VLAN Request*).
  - **IEEE LLDP-MED:** Decodificación de TLV específica de red con OUI `00-12-BB` Subtipo 2 (*Network Policy para Voice/Voice Signaling*), capturando VLAN ID, prioridad 802.1p (CoS) y modo etiquetado.
  - **Huawei LLDP:** Soporte para OUI propietaria `00-E0-FC` Subtipo 1 para extracción de Voice VLAN.
- **Topología Daisy-Chain:** Identificación y mapeo de escenarios donde el equipo de diagnóstico está conectado en cascada detrás de un teléfono IP (*Switch -> Teléfono IP -> PC*).
- **Filtro de Auto-Reflexión:** Descarte estricto de anuncios CDP/LLDP salientes generados por la propia tarjeta del equipo de prueba, evitando bucles de auto-descubrimiento.
- **Veredictos de Switchport:** Evaluación automática del puerto en `PASS`, `WARN` o `FAIL` según el alineamiento de VLAN de datos vs voz.
- **Etiquetado de Rosetas (`--tag`):** Soporte para asociar un identificador de roseta o toma física (ej. `--tag "PISO2-VOZ-04"`) directamente en los reportes exportados.

#### 2. 🛡️ EDR Hardening y Modo Silencioso en Windows (Cero Alertas de TI)
- **Eliminación Total de Ventanas Emergentes:**
  - Se sustituyó la consulta recurrente de puerta de enlace por una llamada en memoria a la API nativa de Windows **`iphlpapi.dll`** (`GetBestRoute`) mediante llamadas syscall de Go puro.
  - **0 procesos PowerShell**, 0 consolas emergentes parpadeando cada 2.5s y ejecución en `< 1 µs`, eliminando cualquier detección o alerta de seguridad corporativa (EDR).
- **Aislamiento de Sesiones ETW:**
  - Procesos de captura encapsulados con flags de kernel `CREATE_NO_WINDOW (0x08000000)` y `HideWindow: true`.
  - Eliminación de banderas sospechosas como `-ExecutionPolicy Bypass`.
  - Compilación con flag de enlace `-H=windowsgui` para supresión absoluta de consolas auxiliares.

#### 3. 🎨 Rediseño Visual Studio & Anti-Slop (Liquid Glass Obsidiana)
- **Identidad Gráfica:** Fondo obsidiana/grafito con efectos de desenfoque multicapa (`backdrop-filter: blur(16px)`) y acento corporativo pastel sobrio **`#8666b0`** (adiós al degradado violeta genérico de IA).
- **Iconografía 100% SVG Vectorial:** Sustitución de todos los emojis por iconos vectoriales monocromáticos que heredan dinámicamente el estilo `currentColor` de la interfaz.
- **Diseño a 2 Columnas Reales (1 Switch por lado):**
  - Resolución de ventana optimizada a **1120 x 780 px**.
  - Rejilla simétrica `1fr 1fr` que aprovecha el espacio horizontal.
  - Arquitectura esquemática de **2 canales por tarjeta**:
    - **Canal izquierdo (protagonistas):** Datos de VLAN Nativa/Acceso y VLAN de Voz con insignias contrastadas.
    - **Canal derecho:** Nombre del switch, puerto, IP de administración, modelo y capacidades.
  - Centrado armónico del mensaje de estado vacío cubriendo ambas columnas.
- **Monitor de Actividad y Barra Dinámica:** Sustitución de la barra estática por un monitor de paquetes en tiempo real con micro-mensajes contextuales informativos (*"Sintonizando interfaz..."*, *"Escuchando tramas multicast..."*, *"Aguardando anuncio de switch..."*).
- **Grafo Topológico Interactivo:** Representación en canvas HTML5 donde el switch descubierto se sitúa como nodo central de distribución y cada puerto como ramificación dinámica.
- **Soporte Bilingüe (i18n):** Selector dinámico de idioma (Español / Inglés), modo de alto contraste y atajos de teclado en el nuevo modal de Ajustes.

#### 4. ⚡ Resiliencia Hot-Plug y Parada Inmediata
- **Botón de Detención Manual (`[Detener]`):** Nuevo control visual y endpoint `/api/stop` para abortar escaneos en cualquier instante sin esperar el tiempo límite de 62 segundos.
- **Auto-Extracción Inmediata al Inicio:** Detección de cable conectado desde el primer milisegundo de ejecución si el conector ya estaba enchufado antes de abrir la aplicación.
- **Estabilización Auto-MDIX:** Ventana de gracia de 1.5s para evitar falsos positivos o errores mientras la tarjeta de red negocia velocidad o duplex.
- **Tolerancia a Desconexión en Caliente:** Cierre de sesiones ETW protegido con `SilentlyContinue` para tolerar desconexiones físicas abruptas de cable a mitad del escaneo sin arrojar excepciones.

---

### 📦 Archivos Adjuntos y Descargas

- **`sre-network-discovery-v2.1.0-windows-x64.zip`**: Ejecutable portable para Windows 10/11 x64 (modo silencioso sin consola, icono oficial, manifiesto UAC para privilegios de administrador y README).
- **`sre-network-discovery-v2.1.0-linux-x64.tar.gz`**: Binario compilado para entornos Linux x64 (AF_PACKET nativo).

### ⚙️ Instrucciones Rápidas de Uso en Windows
1. Descarga y descomprime `sre-network-discovery-v2.1.0-windows-x64.zip`.
2. Haz doble clic en `sre-network-discovery.exe` (o *Clic derecho → Ejecutar como Administrador*).
3. Se abrirá la interfaz de escritorio en modo silencioso.
4. Conecta el cable de red a la toma o switch: la auto-extracción comenzará inmediatamente y extraerá tanto la VLAN de Datos como la de Voz.
