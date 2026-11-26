# NetPulse — Plan Maestro & Arquitectura Técnica

> **Visión:** Suite de escritorio ultrarrápida para Windows 10/11 (**100% local-first, 0% telemetría, 0% cuentas, portable para Discord**) que combina Diagnóstico Dual-Hop & Bufferbloat, Radar LAN estilo Fing, Analizador de Espectro Wi-Fi, Optimizador DNS/UPnP y Piloto de Router con Rollback seguro.

---

## 1. Filosofía Innegociable (Zero-Trust / Zero-Data)

1. **0% Telemetría / 0% Nube:** Ningún dato de red (IPs, MACs, SSIDs, dispositivos descubiertos o métricas) sale de la PC del usuario.
2. **Modo Discord / Streamer Integrado:** Toggle global (`PrivacyMode: true` por defecto) que censura en backend (`internal/privacy/redact.go`) IPs, MACs/BSSIDs y SSIDs antes de mostrarlos en pantalla o copiar reportes, evitando *doxxeo* accidental.
3. **Portable Real:** Binario único `NetPulse.exe` compilado con `CGO_ENABLED=0` sobre WebView2 nativo de Windows, sin dependencias externas (ni Python, ni Node, ni Npcap, ni MSVC C++) y **sin compresión UPX** para evitar falsos positivos heurísticos en Windows Defender.
4. **Privilegios Mínimos:** El dashboard, ping ICMP, detección de red, Radar LAN y lector Wi-Fi corren con usuario estándar (sin UAC). Solo se pide elevación UAC puntual bajo demanda al aplicar DNS en el sistema.

---

## 2. Stack Técnico Verificado

| Capa | Tecnología | Justificación |
| :--- | :--- | :--- |
| **Core / Backend** | **Go 1.27+ (`CGO_ENABLED=0`) + Wails v2** | Compila en segundos a `.exe` nativo, concurrencia ideal para radar LAN y llamadas Win32 directas vía `golang.org/x/sys/windows`. |
| **Frontend / UI** | **React 19 + TypeScript + Tailwind CSS v4 + Lucide** | Interfaz *Tactical Telemetry Dark* (`#090D16`), fuente `Inter` + monospace tabular para telemetría sobre WebView2. |
| **APIs Nativas Win32** | `iphlpapi.dll`, `wlanapi.dll`, `advapi32.dll` | Cero parseo de consola (`netsh`/`ping`/`ipconfig`) que se rompe según el idioma de Windows. |
| **Bóveda Local** | **Windows Credential Manager (`advapi32.dll` / DPAPI)** | Credenciales opcionales de router cifradas por usuario en el llavero oficial de Windows; nunca en texto plano. |
| **Persistencia** | `%APPDATA%\NetPulse\config.json` | Preferencias no sensibles y alias de dispositivos con escritura atómica. |

---

## 3. Trampas Reales de Windows 11 Previstas por Diseño

1. **Bloqueo Wi-Fi en Windows 11 24H2+ (Build 26100+):** `WlanGetNetworkBssList` devuelve `ERROR_ACCESS_DENIED` si los servicios de ubicación para apps de escritorio están desactivados. **Mitigación:** detector automático en Fase 3 con aviso transparente y botón directo a `ms-settings:privacy-location`.
2. **PC conectada solo por Ethernet:** En ausencia de interfaz Wi-Fi física, NetPulse degrada con elegancia solo la pestaña de Espectro Wi-Fi manteniendo 100% operativas las demás herramientas.
3. **Falsos positivos de Antivirus / Defender:** Sin `UPX` y sin escaneos agresivos de miles de puertos; descubrimiento local por protocolos legítimos (ARP, mDNS, SSDP, NetBIOS).
4. **Módems cerrados de ISP:** Sin *web scrapers* frágiles; motor híbrido **Co-Piloto Universal + Auto-Piloto Seguro** solo para APIs limpias (OpenWrt, MikroTik, TR-064).

---

## 4. Las 6 Vistas del Producto

1. **Centro de Mando (Dashboard):** Salud de red (`0–100`), veredicto automático *¿Es mi Wi-Fi o mi Proveedor?*, gráfico Dual-Hop en tiempo real (Salto 1 Router vs Salto 2 Internet) cada 500 ms, test de Bufferbloat (`A+` a `F`) y barra de topología.
2. **Radar LAN:** Descubrimiento concurrente ARP + mDNS + SSDP/UPnP + NetBIOS, base OUI offline, alias editables y detección de dispositivos nuevos/intrusos.
3. **Espectro Wi-Fi:** Lectura nativa `wlanapi.dll`, curvas de solapamiento 2.4 GHz (`1/6/11`) y 5 GHz (separando canales DFS), cálculo físico de interferencia en mW.
4. **DNS & UPnP:** Benchmark paralelo de DNS (ISP vs Cloudflare/Google/Quad9/AdGuard/OpenDNS), cambio con 1 clic + rollback a DHCP y auditoría/cierre de puertos UPnP huérfanos.
5. **Piloto Router:** Co-Piloto universal con guía paso a paso + medición Antes/Después y conectores Auto-Piloto seguros con respaldo previo.
6. **Reporte Discord:** Generador de tarjeta/Markdown 100% anonimizado para compartir en Discord sin filtrar IPs ni ubicación.

---

## 5. Hoja de Ruta por Fases

| Fase | Nombre | Alcance & Entregables | Estado |
| :--- | :--- | :--- | :--- |
| **Fase 0** | **Cimientos & Shell UI** | Toolchain Go + Node + Wails (`CGO_ENABLED=0`), shell oscura, `%APPDATA%\NetPulse\config.json`, bóveda `advapi32.dll` y escudo `internal/privacy/redact.go`. | **COMPLETADA** |
| **Fase 1** | **Motor Dual-Hop & Bufferbloat** | Detección nativa `GetAdaptersAddresses`, ICMP sin Admin (`IcmpSendEcho`), monitor Dual-Hop 500 ms con veredicto automático Wi-Fi vs ISP, test de Bufferbloat (`A+`–`F`) y gráfico SVG en vivo. | **COMPLETADA** |
| **Fase 2** | **Radar LAN (Estilo Fing)** | Barrido concurrente `SendARP` + `GetIpNetTable` + mDNS + SSDP + NetBIOS, base IEEE OUI offline embebida, inferencia de equipo, alias y alerta de intrusos. | **COMPLETADA** |
| **Fase 3** | **Espectro Wi-Fi & Canales** | Binding `wlanapi.dll`, guardia Win11 24H2, motor matemático RF en mW, curvas 2.4/5 GHz, filtro no-DFS y modo Ethernet-only elegante. | **COMPLETADA** |
| **Fase 4** | **Optimizador & Piloto Router** | Benchmark DNS paralelo, aplicador UAC puntual + Rollback DHCP en 1 clic, auditor UPnP, Co-Piloto universal y drivers Auto-Piloto (OpenWrt / MikroTik / TR-064). | **COMPLETADA** |
| **Fase 5** | **Modo Discord & Release** | Generador de reporte Discord censurado, tarjeta PNG, `.zip` portable + instalador Inno Setup 7 y checksum SHA-256 (`v1.0.0`). | **COMPLETADA** |
| **Release v1.1** | **Hardening, Tray & Plus** | Icono custom `.ico`, Log de Incidentes, Regiones Gaming, Traceroute Win32, OUI ampliado + servicios + WoL, Wi-Fi 6 GHz, SSH TOFU OpenWrt, Bandeja Win32 + watcher de red, PNG al portapapeles y GitHub Actions CI. | **COMPLETADA** |
| **Release v1.2** | **Pro Edition (5 Bloques)** | Instancia única + autostart Win32 + ES/EN + tema claro/oscuro + updater opt-in + atajos; MTR continuo + CGNAT/MTU/IPv6 + histórico 30d/CSV/HTML ISP; monitor TCP/UDP por proceso sin drivers; perfiles LAN + presencia + alerta MAC Gateway; manifiestos WinGet/Scoop. | **COMPLETADA** |
