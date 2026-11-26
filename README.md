# NetPulse v1.2.0

[![Release](https://img.shields.io/github/v/release/Patoma89/netpulse?style=flat-square&color=06B6D4)](https://github.com/Patoma89/netpulse/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-10B981?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Windows-10%20%2F%2011%20x64-0EA5E9?style=flat-square)]()
[![Zero Telemetry](https://img.shields.io/badge/Telemetry-0%25%20Local--First-10B981?style=flat-square)]()

> **Web oficial & Descargas:** https://patoma89.github.io/netpulse/  
> **Último Release:** https://github.com/Patoma89/netpulse/releases/latest

Suite de escritorio ultrarrápida para **Windows 10 / 11 (x64)** que combina diagnóstico de red en tiempo real, MTR continuo, auditoría CGNAT/MTU/IPv6, radar LAN estilo Fing, analizador de espectro Wi-Fi (2.4/5/6 GHz), monitor de conexiones por proceso sin drivers, optimizador DNS/UPnP, piloto de router y exportador seguro para Discord.

### Descarga Rápida (`v1.2.0`)

| Opción | Archivo / Comando | Descripción |
| :--- | :--- | :--- |
| **Instalador Windows** | [`NetPulse-v1.2.0-Setup.exe`](https://github.com/Patoma89/netpulse/releases/download/v1.2.0/NetPulse-v1.2.0-Setup.exe) | Instalador moderno Inno Setup (usuario estándar, sin Admin). |
| **Portable (.zip)** | [`NetPulse-v1.2.0-win64-portable.zip`](https://github.com/Patoma89/netpulse/releases/download/v1.2.0/NetPulse-v1.2.0-win64-portable.zip) | Binario único sin instalación + `LEEME.txt`. |
| **Checksums** | [`SHA256SUMS.txt`](https://github.com/Patoma89/netpulse/releases/download/v1.2.0/SHA256SUMS.txt) | Huellas SHA-256 verificables en PowerShell. |

---

## Filosofía Zero-Trust / Zero-Data

- **0% Telemetría, 0% Nube, 0% Cuentas:** Todo corre 100% en tu PC. No sale ningún dato de red, IP, MAC, SSID ni lista de dispositivos hacia servidores externos.
- **Modo Discord (Censura Integrada):** Activado por defecto (`PrivacyMode: true`). Oculta automáticamente tu IP pública/local, puerta de enlace, direcciones MAC/BSSID y nombre del Wi-Fi (SSID) antes de mostrar capturas o copiar reportes en Discord.
- **Privilegios Mínimos:** El dashboard Dual-Hop, Bufferbloat, Radar LAN, Espectro Wi-Fi y Auditoría UPnP corren como usuario estándar (sin Administrador). Solo se solicita elevación UAC puntual en el instante en que aplicás o restaurás servidores DNS.
- **Bóveda Cifrada Local:** Si conectás un router compatible en Auto-Piloto, las credenciales se guardan únicamente en **Windows Credential Manager** (cifrado DPAPI atado a tu usuario local).

---

## Módulos Incluidos

1. **Centro de Mando (Dual-Hop + Bufferbloat + Regiones + Traceroute):**
   - Ping ICMP nativo simultáneo (`iphlpapi.dll` sin Admin) cada `500 ms` al **Router (Salto 1)** y a **Internet (Salto 2)** con **Historial de Incidentes** (microcortes y picos).
   - Veredicto automático en vivo: identifica al instante si el lag está en tu **Wi-Fi/LAN** o en tu **Proveedor (ISP)**.
   - Benchmark multi-región para Gaming (Buenos Aires, São Paulo, Miami, Virginia, Europa) y **Traceroute ICMP Win32 sin Admin**.
   - Test de **Bufferbloat (`A+` a `F`)** midiendo latencia bajo saturación de descarga y subida.
2. **Radar LAN (Estilo Fing):**
   - Barrido concurrente `SendARP` + `GetIpNetTable` + mDNS + SSDP/UPnP + NetBIOS.
   - Base de fabricantes **IEEE OUI offline** ampliada, detección de MACs aleatorias privadas, alias editables, alerta de intrusos, **inspección suave de puertos locales** y **Wake-on-LAN (Magic Packet)**.
3. **Espectro Wi-Fi & Recomendador Matemático (2.4 / 5 / 6 GHz):**
   - Binding Win32 directo a `wlanapi.dll` (sin parsear `netsh`, compatible con cualquier idioma de Windows).
   - Cálculo físico sumando potencia vecina en milivatios ($P_{\text{mW}} = 10^{\text{RSSI}/10}$) y solapamiento espectral.
   - En **2.4 GHz** recomienda exclusivamente canales limpios (`1, 6, 11`); en **5 GHz** excluye canales **DFS (`52–144`)**; en **6 GHz** evalúa canales PSC limpios (Wi-Fi 6E / 7).
   - Degradación elegante si la PC está conectada únicamente por cable Ethernet.
4. **DNS Turbo & Auditoría UPnP:**
   - Benchmark DNS UDP paralelo (Cloudflare, Google, Quad9, AdGuard, OpenDNS vs DNS actual).
   - Cambio en 1 clic con elevación UAC puntual y **Rollback a DHCP** inmediato.
   - Auditoría SSDP/SOAP de puertos UPnP expuestos en tu router con cierre en 1 clic.
5. **Piloto de Router:**
   - **Co-Piloto Universal:** detecta tu gateway/marca, abre el panel local y compara métricas **Antes vs. Después**.
   - **Auto-Piloto Seguro:** conectores nativos para **OpenWrt (SSH/UCI con verificación TOFU)**, **MikroTik (RouterOS REST API)** y **TR-064 (SOAP)** con validación de IPs privadas, bloqueo DFS y **Rollback al canal anterior**.
6. **Tarjeta & Reporte para Discord + Bandeja Windows + UX Pro:**
   - Exporta en 1 clic un bloque Markdown o copia/descarga la tarjeta visual **PNG (1200×660)** 100% anonimizada directamente al portapapeles.
   - Minimizado opcional a la bandeja del sistema Win32, **instancia única**, **inicio con Windows** (`--minimized` sin Admin), **ES/EN**, **tema oscuro/claro**, atajos `Ctrl+Shift+D` (Modo Discord) y `F5` (refrescar), y buscador manual opt-in de actualizaciones.
7. **Diagnóstico Pro Adicional (v1.2.0):**
   - **MTR Continuo (estilo WinMTR)** + **Auditoría ISP (CGNAT / Doble NAT / Path MTU / IPv6)** + **Histórico persistente de 30 días** exportable a **CSV/HTML** para reclamos técnicos al ISP.
   - **Monitor de Conexiones por Proceso (sin drivers)** vía `GetExtendedTcpTable` / `GetExtendedUdpTable` para detectar qué `.exe` satura la red.
   - **Perfiles de Red LAN**, conteo de presencia y **alerta de cambio de MAC del Gateway** (protección contra conflictos IP / ARP spoofing).
   - Manifiestos **WinGet** (`packaging/winget/`) y **Scoop** (`packaging/scoop/`).

---

## Notas Importantes para Windows 10 / 11

### 1. Permiso de Ubicación Wi-Fi en Windows 11 (24H2+)
Desde **Windows 11 24H2**, Microsoft bloquea la lectura de redes Wi-Fi cercanas (`wlanapi.dll`) si los *Servicios de Ubicación para aplicaciones de escritorio* están apagados. Si ves el aviso en **Espectro Wi-Fi**, hacé clic en **Abrir Configuración de Ubicación** y habilitalo para aplicaciones de escritorio (NetPulse nunca envía ni guarda tu ubicación).

### 2. Aviso de Windows SmartScreen (Ejecutable Comunitario sin Firma EV)
NetPulse es un proyecto comunitario gratuito sin certificado comercial EV de Microsoft. La primera vez que alguien lo descargue desde Discord o GitHub, Windows Defender SmartScreen puede mostrar *"Windows protegió su PC"*:
1. Hacé clic en **Más información**.
2. Hacé clic en **Ejecutar de todas formas**.

Para verificar que el archivo no fue alterado, comprobá su huella **SHA-256** contra `release/SHA256SUMS.txt` desde PowerShell:
```powershell
Get-FileHash .\NetPulse.exe -Algorithm SHA256
```

*(Nota: el binario se compila sin compresión `UPX` para evitar falsos positivos heurísticos.)*

---

## Compilar desde el Código Fuente

Requisitos en Windows 10/11 x64: **Go 1.26+**, **Node.js LTS**, **Wails CLI v2** y **WebView2 Runtime** (preinstalado en Windows 11). Opcional: **Inno Setup 7** para el instalador.

```powershell
$env:CGO_ENABLED = "0"
go test -count=1 ./...
powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1
```

Los artefactos finales se generan en `release/`:
- `NetPulse-v1.2.0-win64-portable.zip`
- `NetPulse-v1.2.0-Setup.exe`
- `SHA256SUMS.txt`
