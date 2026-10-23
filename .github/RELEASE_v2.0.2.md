## SRE Network Discovery v2.0.2

Esta versión incluye importantes mejoras operativas, un nuevo formato de exportación diseñado para correos corporativos, detección automática manos libres de cables de red y optimizaciones de compatibilidad con switches Huawei y Cisco.

---

### 🚀 Novedades y Correcciones Principales

#### 1. 📧 Nuevo Formato de Exportación Markdown en "Cápsula" (Compatible con Outlook)
- Las tablas Markdown tradicionales suelen romperse o desbordarse al ser copiadas en Outlook o clientes de correo corporativos.
- Ahora, la exportación a Markdown genera un **listado en cápsulas individuales con viñetas por punto de red**, limpio, legible y listo para pegar en correos de soporte o tickets:
  ```markdown
  Punto de red n°1:
   - Adaptador local: Ethernet
   - Switch Name: SW-PISO2-DIST01
   - IP Gestion: 10.20.30.1
   - Puerto Switch: GigabitEthernet1/0/18
   - VLAN: VLAN 40
   - Modelo: WS-C2960X-48FPS-L
   - Protocolo: CDP + LLDP
   - Detecciones: 2
   - Ultima Vez: 12:05:30
  ```

#### 2. ⚡ "Auto-Extracción de datos" Activada por Defecto (Hands-Free)
- El modo automático ha sido renombrado a **"Auto-Extracción de datos"** y viene **marcado (`checked`) por defecto** al iniciar la aplicación.
- **Monitoreo Reactivo:** La interfaz detecta automáticamente las transiciones físicas de cable (*Carrier UP*): al enchufar el conector RJ45 a una roseta o switch, valida los parámetros de red y **dispara el escaneo de 60s sin necesidad de pulsar botones**. Al desenchufar, queda en espera del siguiente puerto.

#### 3. 🔀 Fusión Inteligente de Protocolos (CDP + LLDP)
- Solucionado el problema por el cual switches que emitían anuncios concurrentes en ambos protocolos generaban dos tarjetas (una completa y otra vacía).
- Ahora el motor correlaciona ambos anuncios por interfaz física y genera **una única tarjeta enriquecida (`CDP + LLDP`)** con toda la telemetría unificada.

#### 4. 🏢 Soporte Completo para Switches Huawei (LLDP)
- Decodificación de modelos de hardware y sistemas operativos desde el campo *System Description* para switches Huawei VRP (series S5700, S5720, S6700, CloudEngine, etc.).
- Extracción robusta de IPs de gestión en TLV 8 (IPv4 e IPv6) y de nombres de puerto en *Port Description*.

#### 5. 🛡️ Privacidad de Red y Apertura Automática
- La interfaz gráfica ahora se abre de manera automática al iniciar el ejecutable en Windows (usando Edge/Chrome en modo app o el navegador predeterminado).
- Se eliminaron las direcciones IP fijas de desarrollo de los mensajes de consola.
- La aplicación se enlaza por defecto a `127.0.0.1`, evitando alertas o bloqueos por parte del Firewall de Windows.

#### 6. 🎨 Banner Animado y Saneamiento del Repositorio
- Se integró un banner vectorial animado SVG nativo con estética cyberpunk/glassmorphism en la portada del proyecto.
- Se eliminaron del repositorio y se bloquearon en `.gitignore` los archivos markdown de agentes (`AGENTS.md`, `SKILL.md`, etc.).

---

### 📦 Archivos Adjuntos y Descargas

- **`sre-network-discovery-v2.0.2-windows-x64.zip`**: Ejecutable portable para Windows 10/11 x64 (incluye icono integrado, manifiesto UAC para privilegios de administrador y README).
- **`sre-network-discovery-v2.0.2-linux-x64.tar.gz`**: Binario compilado para entornos Linux x64.

### ⚙️ Instrucciones Rápidas de Uso en Windows
1. Descarga y descomprime `sre-network-discovery-v2.0.2-windows-x64.zip`.
2. Haz doble clic en `sre-network-discovery.exe` (o *Clic derecho → Ejecutar como Administrador*).
3. Se abrirá automáticamente la ventana de la aplicación.
4. Con **Auto-Extracción de datos** activo, conecta el cable de red al puerto que deseas auditar para comenzar la captura de forma automática.

