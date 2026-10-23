## SRE Network Discovery v2.0.1 (Hotfix & Hardware Support)

Versión de estabilización y compatibilidad multivendor. Corrige errores críticos de inicialización en la interfaz gráfica, añade soporte completo para switches Huawei y unifica la captura simultánea de CDP y LLDP.

---

### 🐛 Correcciones Críticas de Estabilidad

- **Solución al Desplegable de Adaptadores en Blanco**:
  - Resuelto el error fatal de JavaScript (*Temporal Dead Zone* en `isScanning`) que interrumpía la ejecución del script al cargar la página e impedía listar las interfaces de red.
  - Resuelto el pánico en el motor Go provocado por un contexto nulo en la resolución DNS (`probeDNS`).
  - Añadido botón interactivo de refresco dinámico (**🔄 Actualizar adaptadores**) y estilos de contraste optimizados para menús desplegables.

- **Apertura Automática de la Interfaz en Windows**:
  - Al ejecutar `sre-network-discovery.exe` con doble clic, ahora se lanza de forma automática la interfaz gráfica (en modo aplicación sin marcos mediante Edge/Chrome o a través del navegador predeterminado del sistema).

- **Privacidad y Eliminación de Alertas de Firewall**:
  - Se eliminaron las direcciones IP fijas de desarrollo de los registros de consola.
  - El servidor web se enlaza de manera segura a `127.0.0.1` por defecto, eliminando las advertencias de solicitud de permisos del Firewall de Windows.

---

### ⚡ Mejoras de Reconocimiento y Compatibilidad

- **Soporte Completo para Switches Huawei (LLDP)**:
  - Corregido el problema donde los switches Huawei aparecían con datos en blanco.
  - Extracción inteligente del modelo de switch y sistema operativo desde el campo *System Description* (series S5700, S5720, S6700, CloudEngine, etc.).
  - Decodificación robusta de la IP de gestión (TLV 8) y nombres de interfaz desde *Port Description*.

- **Fusión Inteligente de Anuncios Concurrente (CDP + LLDP)**:
  - Si un switch emite tramas en ambos protocolos por el mismo cable, el sistema las consolida en una única tarjeta completa (**`CDP + LLDP`**), evitando tarjetas duplicadas o vacías.

- **Captura Resiliente en Entornos Virtualizados (`BothPhysicalAndSwitch`)**:
  - El proveedor ETW ahora captura tramas tanto en adaptadores físicos como en puentes virtuales, permitiendo operar en equipos con Hyper-V, WSL2, Docker o aislamiento de núcleo de Windows activo.

---

### 📦 Descargas

- **`sre-network-discovery-v2.0.1-windows-x64.zip`**: Binario portable para Windows 10/11 x64 (autoelevable con icono oficial).
- **`sre-network-discovery-v2.0.1-linux-x64.tar.gz`**: Binario para Linux x64.
