export type NavSection =
  | 'dashboard'
  | 'radar'
  | 'wifi'
  | 'processes'
  | 'optimizer'
  | 'router'
  | 'discord';

export interface SectionMeta {
  id: NavSection;
  label: string;
  title: string;
  subtitle: string;
  phase: string;
}

export const SECTIONS: Record<NavSection, SectionMeta> = {
  dashboard: {
    id: 'dashboard',
    label: 'Centro de Mando',
    title: 'Centro de Mando',
    subtitle: 'Dual-Hop en vivo, MTR continuo, Auditoría CGNAT/MTU/IPv6 e histórico 30 días',
    phase: 'v1.2 · Pro',
  },
  radar: {
    id: 'radar',
    label: 'Radar LAN',
    title: 'Radar LAN',
    subtitle: 'Perfiles de red, alerta MAC del Gateway, presencia y Wake-on-LAN',
    phase: 'v1.2 · Pro',
  },
  wifi: {
    id: 'wifi',
    label: 'Espectro Wi-Fi',
    title: 'Espectro Wi-Fi',
    subtitle: 'Análisis nativo Win32 WLAN (2.4 / 5 / 6 GHz), solapamiento RF y recomendación',
    phase: 'v1.2 · Pro',
  },
  processes: {
    id: 'processes',
    label: 'Conexiones',
    title: 'Conexiones por Proceso',
    subtitle: 'Sockets TCP/UDP activos e I/O por programa en Windows sin drivers de kernel',
    phase: 'v1.2 · Pro',
  },
  optimizer: {
    id: 'optimizer',
    label: 'DNS & UPnP',
    title: 'DNS Turbo & Auditoría UPnP',
    subtitle: 'Benchmark DNS paralelo, cambio con 1 clic, rollback DHCP y puertos abiertos',
    phase: 'v1.2 · Pro',
  },
  router: {
    id: 'router',
    label: 'Piloto Router',
    title: 'Piloto de Router',
    subtitle: 'Co-Piloto universal para módems ISP y Auto-Piloto con SSH TOFU y rollback',
    phase: 'v1.2 · Pro',
  },
  discord: {
    id: 'discord',
    label: 'Reporte Discord',
    title: 'Tarjeta para Discord',
    subtitle: 'Exportador Markdown e imagen PNG 100% anonimizada para compartir sin riesgos',
    phase: 'v1.2 · Pro',
  },
};
