export namespace config {
	
	export class NetworkProfile {
	    id: string;
	    name: string;
	    gatewayIp: string;
	    gatewayMac: string;
	    subnet: string;
	    trustedMacs: Record<string, boolean>;
	    lastSeen: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.gatewayIp = source["gatewayIp"];
	        this.gatewayMac = source["gatewayMac"];
	        this.subnet = source["subnet"];
	        this.trustedMacs = source["trustedMacs"];
	        this.lastSeen = source["lastSeen"];
	    }
	}
	export class KnownDevice {
	    alias: string;
	    trusted: boolean;
	    category: string;
	    firstSeen: string;
	    lastSeen: string;
	    seenCount: number;
	    online: boolean;
	    lastTransition: string;
	
	    static createFrom(source: any = {}) {
	        return new KnownDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.trusted = source["trusted"];
	        this.category = source["category"];
	        this.firstSeen = source["firstSeen"];
	        this.lastSeen = source["lastSeen"];
	        this.seenCount = source["seenCount"];
	        this.online = source["online"];
	        this.lastTransition = source["lastTransition"];
	    }
	}
	export class AppConfig {
	    privacyMode: boolean;
	    language: string;
	    theme: string;
	    pingTarget: string;
	    knownDevices: Record<string, KnownDevice>;
	    networkProfiles: Record<string, NetworkProfile>;
	    minimizeToTray: boolean;
	    backgroundRadarAlert: boolean;
	    startWithWindows: boolean;
	    updateRepo: string;
	    sshHostFingerprints: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.privacyMode = source["privacyMode"];
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.pingTarget = source["pingTarget"];
	        this.knownDevices = this.convertValues(source["knownDevices"], KnownDevice, true);
	        this.networkProfiles = this.convertValues(source["networkProfiles"], NetworkProfile, true);
	        this.minimizeToTray = source["minimizeToTray"];
	        this.backgroundRadarAlert = source["backgroundRadarAlert"];
	        this.startWithWindows = source["startWithWindows"];
	        this.updateRepo = source["updateRepo"];
	        this.sshHostFingerprints = source["sshHostFingerprints"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace diagnostics {
	
	export class BufferbloatResult {
	    grade: string;
	    severity: string;
	    summary: string;
	    recommendation: string;
	    target: string;
	    idleMedianMs: number;
	    idleAvgMs: number;
	    downloadMedianMs: number;
	    downloadDeltaMs: number;
	    downloadMbps: number;
	    downloadLossPct: number;
	    uploadMedianMs: number;
	    uploadDeltaMs: number;
	    uploadMbps: number;
	    uploadLossPct: number;
	    uploadSkipped: boolean;
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new BufferbloatResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.grade = source["grade"];
	        this.severity = source["severity"];
	        this.summary = source["summary"];
	        this.recommendation = source["recommendation"];
	        this.target = source["target"];
	        this.idleMedianMs = source["idleMedianMs"];
	        this.idleAvgMs = source["idleAvgMs"];
	        this.downloadMedianMs = source["downloadMedianMs"];
	        this.downloadDeltaMs = source["downloadDeltaMs"];
	        this.downloadMbps = source["downloadMbps"];
	        this.downloadLossPct = source["downloadLossPct"];
	        this.uploadMedianMs = source["uploadMedianMs"];
	        this.uploadDeltaMs = source["uploadDeltaMs"];
	        this.uploadMbps = source["uploadMbps"];
	        this.uploadLossPct = source["uploadLossPct"];
	        this.uploadSkipped = source["uploadSkipped"];
	        this.completedAt = source["completedAt"];
	    }
	}
	export class DualHopSample {
	    timestamp: number;
	    seq: number;
	    routerMs: number;
	    routerOk: boolean;
	    internetMs: number;
	    internetOk: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DualHopSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.seq = source["seq"];
	        this.routerMs = source["routerMs"];
	        this.routerOk = source["routerOk"];
	        this.internetMs = source["internetMs"];
	        this.internetOk = source["internetOk"];
	    }
	}
	export class StabilityIncident {
	    id: number;
	    timestamp: number;
	    timeLabel: string;
	    hop: string;
	    kind: string;
	    severity: string;
	    durationMs: number;
	    peakMs: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new StabilityIncident(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.timeLabel = source["timeLabel"];
	        this.hop = source["hop"];
	        this.kind = source["kind"];
	        this.severity = source["severity"];
	        this.durationMs = source["durationMs"];
	        this.peakMs = source["peakMs"];
	        this.message = source["message"];
	    }
	}
	export class Verdict {
	    code: string;
	    severity: string;
	    title: string;
	    detail: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new Verdict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.severity = source["severity"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	        this.score = source["score"];
	    }
	}
	export class HopStats {
	    target: string;
	    currentMs: number;
	    avgMs: number;
	    minMs: number;
	    maxMs: number;
	    p95Ms: number;
	    jitterMs: number;
	    lossPct: number;
	    sent: number;
	    lost: number;
	    lastOk: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HopStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.currentMs = source["currentMs"];
	        this.avgMs = source["avgMs"];
	        this.minMs = source["minMs"];
	        this.maxMs = source["maxMs"];
	        this.p95Ms = source["p95Ms"];
	        this.jitterMs = source["jitterMs"];
	        this.lossPct = source["lossPct"];
	        this.sent = source["sent"];
	        this.lost = source["lost"];
	        this.lastOk = source["lastOk"];
	    }
	}
	export class DualHopSnapshot {
	    running: boolean;
	    routerTarget: string;
	    internetTarget: string;
	    router: HopStats;
	    internet: HopStats;
	    verdict: Verdict;
	    samples: DualHopSample[];
	    incidents: StabilityIncident[];
	
	    static createFrom(source: any = {}) {
	        return new DualHopSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.routerTarget = source["routerTarget"];
	        this.internetTarget = source["internetTarget"];
	        this.router = this.convertValues(source["router"], HopStats);
	        this.internet = this.convertValues(source["internet"], HopStats);
	        this.verdict = this.convertValues(source["verdict"], Verdict);
	        this.samples = this.convertValues(source["samples"], DualHopSample);
	        this.incidents = this.convertValues(source["incidents"], StabilityIncident);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class IPv6Status {
	    hasLocalIpv6: boolean;
	    hasGlobalIpv6: boolean;
	    reachable: boolean;
	    rttMs: number;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new IPv6Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasLocalIpv6 = source["hasLocalIpv6"];
	        this.hasGlobalIpv6 = source["hasGlobalIpv6"];
	        this.reachable = source["reachable"];
	        this.rttMs = source["rttMs"];
	        this.detail = source["detail"];
	    }
	}
	export class MTRHopStats {
	    ttl: number;
	    ip: string;
	    sent: number;
	    lost: number;
	    lossPct: number;
	    lastMs: number;
	    bestMs: number;
	    avgMs: number;
	    worstMs: number;
	    jitterMs: number;
	    isTarget: boolean;
	    isPrivate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MTRHopStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ttl = source["ttl"];
	        this.ip = source["ip"];
	        this.sent = source["sent"];
	        this.lost = source["lost"];
	        this.lossPct = source["lossPct"];
	        this.lastMs = source["lastMs"];
	        this.bestMs = source["bestMs"];
	        this.avgMs = source["avgMs"];
	        this.worstMs = source["worstMs"];
	        this.jitterMs = source["jitterMs"];
	        this.isTarget = source["isTarget"];
	        this.isPrivate = source["isPrivate"];
	    }
	}
	export class MTRSnapshot {
	    running: boolean;
	    target: string;
	    startedAt: string;
	    cycles: number;
	    reachedTarget: boolean;
	    worstLossTtl: number;
	    worstLossPct: number;
	    hops: MTRHopStats[];
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new MTRSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.target = source["target"];
	        this.startedAt = source["startedAt"];
	        this.cycles = source["cycles"];
	        this.reachedTarget = source["reachedTarget"];
	        this.worstLossTtl = source["worstLossTtl"];
	        this.worstLossPct = source["worstLossPct"];
	        this.hops = this.convertValues(source["hops"], MTRHopStats);
	        this.summary = source["summary"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MTUStatus {
	    pathMtu: number;
	    standardMtu: number;
	    fragmented: boolean;
	    target: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new MTUStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pathMtu = source["pathMtu"];
	        this.standardMtu = source["standardMtu"];
	        this.fragmented = source["fragmented"];
	        this.target = source["target"];
	        this.detail = source["detail"];
	    }
	}
	export class TraceHop {
	    ttl: number;
	    ip: string;
	    rttMs: number;
	    timedOut: boolean;
	    isTarget: boolean;
	    deltaMs: number;
	
	    static createFrom(source: any = {}) {
	        return new TraceHop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ttl = source["ttl"];
	        this.ip = source["ip"];
	        this.rttMs = source["rttMs"];
	        this.timedOut = source["timedOut"];
	        this.isTarget = source["isTarget"];
	        this.deltaMs = source["deltaMs"];
	    }
	}
	export class NATStatus {
	    kind: string;
	    label: string;
	    detail: string;
	    cgnatHopIp: string;
	    doubleNatHopIp: string;
	    firstHops: TraceHop[];
	
	    static createFrom(source: any = {}) {
	        return new NATStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.cgnatHopIp = source["cgnatHopIp"];
	        this.doubleNatHopIp = source["doubleNatHopIp"];
	        this.firstHops = this.convertValues(source["firstHops"], TraceHop);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NetworkAuditResult {
	    completedAt: string;
	    nat: NATStatus;
	    mtu: MTUStatus;
	    ipv6: IPv6Status;
	    overallNote: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkAuditResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.completedAt = source["completedAt"];
	        this.nat = this.convertValues(source["nat"], NATStatus);
	        this.mtu = this.convertValues(source["mtu"], MTUStatus);
	        this.ipv6 = this.convertValues(source["ipv6"], IPv6Status);
	        this.overallNote = source["overallNote"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RegionPingResult {
	    id: string;
	    name: string;
	    region: string;
	    category: string;
	    host: string;
	    minMs: number;
	    avgMs: number;
	    jitterMs: number;
	    reachable: boolean;
	    grade: string;
	
	    static createFrom(source: any = {}) {
	        return new RegionPingResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.category = source["category"];
	        this.host = source["host"];
	        this.minMs = source["minMs"];
	        this.avgMs = source["avgMs"];
	        this.jitterMs = source["jitterMs"];
	        this.reachable = source["reachable"];
	        this.grade = source["grade"];
	    }
	}
	export class RegionBenchmarkResult {
	    completedAt: string;
	    bestRegion: string;
	    results: RegionPingResult[];
	
	    static createFrom(source: any = {}) {
	        return new RegionBenchmarkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.completedAt = source["completedAt"];
	        this.bestRegion = source["bestRegion"];
	        this.results = this.convertValues(source["results"], RegionPingResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class TracerouteResult {
	    target: string;
	    completedAt: string;
	    reachedTarget: boolean;
	    worstHopTtl: number;
	    worstJumpMs: number;
	    hops: TraceHop[];
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new TracerouteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.completedAt = source["completedAt"];
	        this.reachedTarget = source["reachedTarget"];
	        this.worstHopTtl = source["worstHopTtl"];
	        this.worstJumpMs = source["worstJumpMs"];
	        this.hops = this.convertValues(source["hops"], TraceHop);
	        this.summary = source["summary"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace history {
	
	export class DailySummary {
	    date: string;
	    totalIncidents: number;
	    lossIncidents: number;
	    spikeIncidents: number;
	    worstPeakMs: number;
	
	    static createFrom(source: any = {}) {
	        return new DailySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.totalIncidents = source["totalIncidents"];
	        this.lossIncidents = source["lossIncidents"];
	        this.spikeIncidents = source["spikeIncidents"];
	        this.worstPeakMs = source["worstPeakMs"];
	    }
	}
	export class IncidentRecord {
	    id: number;
	    timestamp: number;
	    timeIso: string;
	    networkName: string;
	    hop: string;
	    kind: string;
	    severity: string;
	    durationMs: number;
	    peakMs: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new IncidentRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.timeIso = source["timeIso"];
	        this.networkName = source["networkName"];
	        this.hop = source["hop"];
	        this.kind = source["kind"];
	        this.severity = source["severity"];
	        this.durationMs = source["durationMs"];
	        this.peakMs = source["peakMs"];
	        this.message = source["message"];
	    }
	}
	export class HistorySnapshot {
	    totalStored: number;
	    retentionDays: number;
	    incidents: IncidentRecord[];
	    daily: DailySummary[];
	
	    static createFrom(source: any = {}) {
	        return new HistorySnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalStored = source["totalStored"];
	        this.retentionDays = source["retentionDays"];
	        this.incidents = this.convertValues(source["incidents"], IncidentRecord);
	        this.daily = this.convertValues(source["daily"], DailySummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class AppInfo {
	    name: string;
	    version: string;
	    os: string;
	    arch: string;
	    configPath: string;
	    zeroTelemetry: boolean;
	    vaultReady: boolean;
	    vaultError: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.configPath = source["configPath"];
	        this.zeroTelemetry = source["zeroTelemetry"];
	        this.vaultReady = source["vaultReady"];
	        this.vaultError = source["vaultError"];
	    }
	}
	export class RedactedPreview {
	    ip: string;
	    mac: string;
	    ssid: string;
	
	    static createFrom(source: any = {}) {
	        return new RedactedPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.mac = source["mac"];
	        this.ssid = source["ssid"];
	    }
	}

}

export namespace netinfo {
	
	export class AdapterInfo {
	    name: string;
	    description: string;
	    mac: string;
	    ipv4: string;
	    prefixLen: number;
	    gateway: string;
	    dnsServers: string[];
	    type: string;
	    linkSpeedMbps: number;
	    ifIndex: number;
	    isUp: boolean;
	    isPrimary: boolean;
	    isVirtual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AdapterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.mac = source["mac"];
	        this.ipv4 = source["ipv4"];
	        this.prefixLen = source["prefixLen"];
	        this.gateway = source["gateway"];
	        this.dnsServers = source["dnsServers"];
	        this.type = source["type"];
	        this.linkSpeedMbps = source["linkSpeedMbps"];
	        this.ifIndex = source["ifIndex"];
	        this.isUp = source["isUp"];
	        this.isPrimary = source["isPrimary"];
	        this.isVirtual = source["isVirtual"];
	    }
	}
	export class NetworkSummary {
	    connected: boolean;
	    primary?: AdapterInfo;
	    adapters: AdapterInfo[];
	    hasWiFiAdapter: boolean;
	    gatewayIp: string;
	    localIp: string;
	    adapterMac: string;
	    adapterName: string;
	    linkType: string;
	    linkSpeedMbps: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.primary = this.convertValues(source["primary"], AdapterInfo);
	        this.adapters = this.convertValues(source["adapters"], AdapterInfo);
	        this.hasWiFiAdapter = source["hasWiFiAdapter"];
	        this.gatewayIp = source["gatewayIp"];
	        this.localIp = source["localIp"];
	        this.adapterMac = source["adapterMac"];
	        this.adapterName = source["adapterName"];
	        this.linkType = source["linkType"];
	        this.linkSpeedMbps = source["linkSpeedMbps"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace optimizer {
	
	export class DNSProvider {
	    id: string;
	    name: string;
	    tagline: string;
	    primary: string;
	    secondary: string;
	    isCurrent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DNSProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.tagline = source["tagline"];
	        this.primary = source["primary"];
	        this.secondary = source["secondary"];
	        this.isCurrent = source["isCurrent"];
	    }
	}
	export class DNSResult {
	    provider: DNSProvider;
	    medianMs: number;
	    minMs: number;
	    avgMs: number;
	    jitterMs: number;
	    successCount: number;
	    totalQueries: number;
	    reachable: boolean;
	    recommended: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DNSResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = this.convertValues(source["provider"], DNSProvider);
	        this.medianMs = source["medianMs"];
	        this.minMs = source["minMs"];
	        this.avgMs = source["avgMs"];
	        this.jitterMs = source["jitterMs"];
	        this.successCount = source["successCount"];
	        this.totalQueries = source["totalQueries"];
	        this.reachable = source["reachable"];
	        this.recommended = source["recommended"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DNSBenchmarkResult {
	    adapterName: string;
	    ifIndex: number;
	    currentDns: string[];
	    fastestId: string;
	    fastestName: string;
	    completedAt: string;
	    results: DNSResult[];
	
	    static createFrom(source: any = {}) {
	        return new DNSBenchmarkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapterName = source["adapterName"];
	        this.ifIndex = source["ifIndex"];
	        this.currentDns = source["currentDns"];
	        this.fastestId = source["fastestId"];
	        this.fastestName = source["fastestName"];
	        this.completedAt = source["completedAt"];
	        this.results = this.convertValues(source["results"], DNSResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class UPnPMapping {
	    index: number;
	    externalPort: number;
	    internalPort: number;
	    protocol: string;
	    internalClient: string;
	    enabled: boolean;
	    description: string;
	    leaseDuration: number;
	    risk: string;
	    riskReason: string;
	
	    static createFrom(source: any = {}) {
	        return new UPnPMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.externalPort = source["externalPort"];
	        this.internalPort = source["internalPort"];
	        this.protocol = source["protocol"];
	        this.internalClient = source["internalClient"];
	        this.enabled = source["enabled"];
	        this.description = source["description"];
	        this.leaseDuration = source["leaseDuration"];
	        this.risk = source["risk"];
	        this.riskReason = source["riskReason"];
	    }
	}
	export class UPnPScanResult {
	    status: string;
	    message: string;
	    gatewayIp: string;
	    deviceName: string;
	    serviceType: string;
	    controlUrl: string;
	    completedAt: string;
	    mappings: UPnPMapping[];
	    highRiskCount: number;
	
	    static createFrom(source: any = {}) {
	        return new UPnPScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.gatewayIp = source["gatewayIp"];
	        this.deviceName = source["deviceName"];
	        this.serviceType = source["serviceType"];
	        this.controlUrl = source["controlUrl"];
	        this.completedAt = source["completedAt"];
	        this.mappings = this.convertValues(source["mappings"], UPnPMapping);
	        this.highRiskCount = source["highRiskCount"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace processes {
	
	export class ConnectionEndpoint {
	    protocol: string;
	    state: string;
	    localPort: number;
	    remoteIp: string;
	    remotePort: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionEndpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.state = source["state"];
	        this.localPort = source["localPort"];
	        this.remoteIp = source["remoteIp"];
	        this.remotePort = source["remotePort"];
	    }
	}
	export class ProcessNetUsage {
	    pid: number;
	    name: string;
	    exePath: string;
	    tcpCount: number;
	    establishedCount: number;
	    udpCount: number;
	    totalConnections: number;
	    ioKbPerSec: number;
	    remoteSample: ConnectionEndpoint[];
	
	    static createFrom(source: any = {}) {
	        return new ProcessNetUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.name = source["name"];
	        this.exePath = source["exePath"];
	        this.tcpCount = source["tcpCount"];
	        this.establishedCount = source["establishedCount"];
	        this.udpCount = source["udpCount"];
	        this.totalConnections = source["totalConnections"];
	        this.ioKbPerSec = source["ioKbPerSec"];
	        this.remoteSample = this.convertValues(source["remoteSample"], ConnectionEndpoint);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProcessSnapshot {
	    capturedAt: string;
	    totalProcesses: number;
	    totalConnections: number;
	    establishedTotal: number;
	    processes: ProcessNetUsage[];
	
	    static createFrom(source: any = {}) {
	        return new ProcessSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capturedAt = source["capturedAt"];
	        this.totalProcesses = source["totalProcesses"];
	        this.totalConnections = source["totalConnections"];
	        this.establishedTotal = source["establishedTotal"];
	        this.processes = this.convertValues(source["processes"], ProcessNetUsage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace radar {
	
	export class Device {
	    key: string;
	    ip: string;
	    mac: string;
	    hostname: string;
	    netbiosName: string;
	    mdnsName: string;
	    ssdpName: string;
	    vendor: string;
	    isRandomizedMac: boolean;
	    category: string;
	    alias: string;
	    trusted: boolean;
	    isNew: boolean;
	    isLocal: boolean;
	    isGateway: boolean;
	    rttMs: number;
	    protocols: string[];
	    services: string[];
	    firstSeen: string;
	    lastSeen: string;
	    seenCount: number;
	    online: boolean;
	    lastTransition: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.ip = source["ip"];
	        this.mac = source["mac"];
	        this.hostname = source["hostname"];
	        this.netbiosName = source["netbiosName"];
	        this.mdnsName = source["mdnsName"];
	        this.ssdpName = source["ssdpName"];
	        this.vendor = source["vendor"];
	        this.isRandomizedMac = source["isRandomizedMac"];
	        this.category = source["category"];
	        this.alias = source["alias"];
	        this.trusted = source["trusted"];
	        this.isNew = source["isNew"];
	        this.isLocal = source["isLocal"];
	        this.isGateway = source["isGateway"];
	        this.rttMs = source["rttMs"];
	        this.protocols = source["protocols"];
	        this.services = source["services"];
	        this.firstSeen = source["firstSeen"];
	        this.lastSeen = source["lastSeen"];
	        this.seenCount = source["seenCount"];
	        this.online = source["online"];
	        this.lastTransition = source["lastTransition"];
	    }
	}
	export class ServicePort {
	    port: number;
	    name: string;
	    open: boolean;
	    rttMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ServicePort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.name = source["name"];
	        this.open = source["open"];
	        this.rttMs = source["rttMs"];
	    }
	}
	export class DeviceServicesResult {
	    deviceKey: string;
	    ip: string;
	    checkedAt: string;
	    services: ServicePort[];
	
	    static createFrom(source: any = {}) {
	        return new DeviceServicesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceKey = source["deviceKey"];
	        this.ip = source["ip"];
	        this.checkedAt = source["checkedAt"];
	        this.services = this.convertValues(source["services"], ServicePort);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GatewayMACAlert {
	    active: boolean;
	    profileId: string;
	    previousMac: string;
	    currentMac: string;
	    detectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new GatewayMACAlert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.profileId = source["profileId"];
	        this.previousMac = source["previousMac"];
	        this.currentMac = source["currentMac"];
	        this.detectedAt = source["detectedAt"];
	    }
	}
	export class ScanResult {
	    subnet: string;
	    scannedHosts: number;
	    durationMs: number;
	    completedAt: string;
	    totalDevices: number;
	    trustedCount: number;
	    unknownCount: number;
	    newCount: number;
	    profileId: string;
	    profileName: string;
	    gatewayMacAlert?: GatewayMACAlert;
	    devices: Device[];
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subnet = source["subnet"];
	        this.scannedHosts = source["scannedHosts"];
	        this.durationMs = source["durationMs"];
	        this.completedAt = source["completedAt"];
	        this.totalDevices = source["totalDevices"];
	        this.trustedCount = source["trustedCount"];
	        this.unknownCount = source["unknownCount"];
	        this.newCount = source["newCount"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.gatewayMacAlert = this.convertValues(source["gatewayMacAlert"], GatewayMACAlert);
	        this.devices = this.convertValues(source["devices"], Device);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace report {
	
	export class DiscordReport {
	    generatedAt: string;
	    version: string;
	    redacted: boolean;
	    healthScore: number;
	    verdictTitle: string;
	    verdictSeverity: string;
	    linkLabel: string;
	    routerLatency: string;
	    internetLatency: string;
	    bufferbloatGrade: string;
	    wifiSummary: string;
	    radarSummary: string;
	    dnsSummary: string;
	    upnpSummary: string;
	    routerSummary: string;
	    redactedFields: string[];
	    markdown: string;
	    plainText: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscordReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.version = source["version"];
	        this.redacted = source["redacted"];
	        this.healthScore = source["healthScore"];
	        this.verdictTitle = source["verdictTitle"];
	        this.verdictSeverity = source["verdictSeverity"];
	        this.linkLabel = source["linkLabel"];
	        this.routerLatency = source["routerLatency"];
	        this.internetLatency = source["internetLatency"];
	        this.bufferbloatGrade = source["bufferbloatGrade"];
	        this.wifiSummary = source["wifiSummary"];
	        this.radarSummary = source["radarSummary"];
	        this.dnsSummary = source["dnsSummary"];
	        this.upnpSummary = source["upnpSummary"];
	        this.routerSummary = source["routerSummary"];
	        this.redactedFields = source["redactedFields"];
	        this.markdown = source["markdown"];
	        this.plainText = source["plainText"];
	    }
	}

}

export namespace router {
	
	export class AutoPilotConfig {
	    driver: string;
	    host: string;
	    port: number;
	    username: string;
	    password?: string;
	    useTls: boolean;
	    saveCredentials: boolean;
	    hasSavedCreds: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AutoPilotConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driver = source["driver"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.useTls = source["useTls"];
	        this.saveCredentials = source["saveCredentials"];
	        this.hasSavedCreds = source["hasSavedCreds"];
	    }
	}
	export class RadioChannelInfo {
	    interfaceName: string;
	    band: string;
	    currentChannel: number;
	    widthMHz: number;
	
	    static createFrom(source: any = {}) {
	        return new RadioChannelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interfaceName = source["interfaceName"];
	        this.band = source["band"];
	        this.currentChannel = source["currentChannel"];
	        this.widthMHz = source["widthMHz"];
	    }
	}
	export class AutoPilotStatus {
	    connected: boolean;
	    driver: string;
	    model: string;
	    message: string;
	    hostFingerprint?: string;
	    radios: RadioChannelInfo[];
	
	    static createFrom(source: any = {}) {
	        return new AutoPilotStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.driver = source["driver"];
	        this.model = source["model"];
	        this.message = source["message"];
	        this.hostFingerprint = source["hostFingerprint"];
	        this.radios = this.convertValues(source["radios"], RadioChannelInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BeforeAfterSnapshot {
	    capturedAt: string;
	    routerAvgMs: number;
	    routerJitterMs: number;
	    routerLossPct: number;
	    internetAvgMs: number;
	    wifiBand: string;
	    wifiChannel: number;
	    wifiCongestion: number;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new BeforeAfterSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capturedAt = source["capturedAt"];
	        this.routerAvgMs = source["routerAvgMs"];
	        this.routerJitterMs = source["routerJitterMs"];
	        this.routerLossPct = source["routerLossPct"];
	        this.internetAvgMs = source["internetAvgMs"];
	        this.wifiBand = source["wifiBand"];
	        this.wifiChannel = source["wifiChannel"];
	        this.wifiCongestion = source["wifiCongestion"];
	        this.score = source["score"];
	    }
	}
	export class BeforeAfterReport {
	    hasBaseline: boolean;
	    baseline: BeforeAfterSnapshot;
	    after?: BeforeAfterSnapshot;
	    deltaRouterMs: number;
	    deltaJitterMs: number;
	    deltaCongestion: number;
	    deltaScore: number;
	    verdict: string;
	
	    static createFrom(source: any = {}) {
	        return new BeforeAfterReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasBaseline = source["hasBaseline"];
	        this.baseline = this.convertValues(source["baseline"], BeforeAfterSnapshot);
	        this.after = this.convertValues(source["after"], BeforeAfterSnapshot);
	        this.deltaRouterMs = source["deltaRouterMs"];
	        this.deltaJitterMs = source["deltaJitterMs"];
	        this.deltaCongestion = source["deltaCongestion"];
	        this.deltaScore = source["deltaScore"];
	        this.verdict = source["verdict"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ChannelBackup {
	    driver: string;
	    host: string;
	    interfaceName: string;
	    band: string;
	    previousChannel: number;
	    appliedChannel: number;
	    savedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ChannelBackup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driver = source["driver"];
	        this.host = source["host"];
	        this.interfaceName = source["interfaceName"];
	        this.band = source["band"];
	        this.previousChannel = source["previousChannel"];
	        this.appliedChannel = source["appliedChannel"];
	        this.savedAt = source["savedAt"];
	    }
	}
	export class ChannelChangeRequest {
	    driver: string;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    useTls: boolean;
	    interfaceName: string;
	    band: string;
	    targetChannel: number;
	    allowDfs: boolean;
	    confirmedDisconnect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelChangeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.driver = source["driver"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.useTls = source["useTls"];
	        this.interfaceName = source["interfaceName"];
	        this.band = source["band"];
	        this.targetChannel = source["targetChannel"];
	        this.allowDfs = source["allowDfs"];
	        this.confirmedDisconnect = source["confirmedDisconnect"];
	    }
	}
	export class ChannelChangeResult {
	    success: boolean;
	    message: string;
	    previousChannel: number;
	    appliedChannel: number;
	    connectivityOk: boolean;
	    backup?: ChannelBackup;
	
	    static createFrom(source: any = {}) {
	        return new ChannelChangeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.previousChannel = source["previousChannel"];
	        this.appliedChannel = source["appliedChannel"];
	        this.connectivityOk = source["connectivityOk"];
	        this.backup = this.convertValues(source["backup"], ChannelBackup);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RouterProfile {
	    detected: boolean;
	    gatewayIp: string;
	    gatewayMac: string;
	    vendor: string;
	    adminUrl: string;
	    httpTitle: string;
	    serverHeader: string;
	    menuGuide: string;
	    suggestedDriver: string;
	    autoPilotSupported: boolean;
	    rec24Channel: number;
	    rec5Channel: number;
	    recDnsName: string;
	    recDnsPrimary: string;
	
	    static createFrom(source: any = {}) {
	        return new RouterProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.detected = source["detected"];
	        this.gatewayIp = source["gatewayIp"];
	        this.gatewayMac = source["gatewayMac"];
	        this.vendor = source["vendor"];
	        this.adminUrl = source["adminUrl"];
	        this.httpTitle = source["httpTitle"];
	        this.serverHeader = source["serverHeader"];
	        this.menuGuide = source["menuGuide"];
	        this.suggestedDriver = source["suggestedDriver"];
	        this.autoPilotSupported = source["autoPilotSupported"];
	        this.rec24Channel = source["rec24Channel"];
	        this.rec5Channel = source["rec5Channel"];
	        this.recDnsName = source["recDnsName"];
	        this.recDnsPrimary = source["recDnsPrimary"];
	    }
	}

}

export namespace updater {
	
	export class UpdateCheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    releaseUrl: string;
	    checksumsUrl: string;
	    publishedAt: string;
	    notes: string;
	    checkedAt: string;
	    status: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.releaseUrl = source["releaseUrl"];
	        this.checksumsUrl = source["checksumsUrl"];
	        this.publishedAt = source["publishedAt"];
	        this.notes = source["notes"];
	        this.checkedAt = source["checkedAt"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}

}

export namespace wifi {
	
	export class AccessPoint {
	    ssid: string;
	    hidden: boolean;
	    bssid: string;
	    vendor: string;
	    band: string;
	    channel: number;
	    centerChannel: number;
	    frequencyMHz: number;
	    widthMHz: number;
	    rssi: number;
	    linkQuality: number;
	    generation: string;
	    security: string;
	    isDfs: boolean;
	    isCurrent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccessPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssid = source["ssid"];
	        this.hidden = source["hidden"];
	        this.bssid = source["bssid"];
	        this.vendor = source["vendor"];
	        this.band = source["band"];
	        this.channel = source["channel"];
	        this.centerChannel = source["centerChannel"];
	        this.frequencyMHz = source["frequencyMHz"];
	        this.widthMHz = source["widthMHz"];
	        this.rssi = source["rssi"];
	        this.linkQuality = source["linkQuality"];
	        this.generation = source["generation"];
	        this.security = source["security"];
	        this.isDfs = source["isDfs"];
	        this.isCurrent = source["isCurrent"];
	    }
	}
	export class BandRecommendation {
	    band: string;
	    currentChannel: number;
	    recommendedChannel: number;
	    currentCongestion: number;
	    bestCongestion: number;
	    improvementPct: number;
	    shouldSwitch: boolean;
	    status: string;
	    headline: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new BandRecommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.band = source["band"];
	        this.currentChannel = source["currentChannel"];
	        this.recommendedChannel = source["recommendedChannel"];
	        this.currentCongestion = source["currentCongestion"];
	        this.bestCongestion = source["bestCongestion"];
	        this.improvementPct = source["improvementPct"];
	        this.shouldSwitch = source["shouldSwitch"];
	        this.status = source["status"];
	        this.headline = source["headline"];
	        this.detail = source["detail"];
	    }
	}
	export class ChannelScore {
	    channel: number;
	    band: string;
	    congestionPct: number;
	    qualityScore: number;
	    interferenceDbm: number;
	    coChannelCount: number;
	    overlapCount: number;
	    isDfs: boolean;
	    isRecommended: boolean;
	    isCurrent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelScore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.band = source["band"];
	        this.congestionPct = source["congestionPct"];
	        this.qualityScore = source["qualityScore"];
	        this.interferenceDbm = source["interferenceDbm"];
	        this.coChannelCount = source["coChannelCount"];
	        this.overlapCount = source["overlapCount"];
	        this.isDfs = source["isDfs"];
	        this.isRecommended = source["isRecommended"];
	        this.isCurrent = source["isCurrent"];
	    }
	}
	export class ScanResult {
	    status: string;
	    errorMessage: string;
	    adapterName: string;
	    connected: boolean;
	    currentSsid: string;
	    currentBssid: string;
	    currentBand: string;
	    currentChannel: number;
	    currentWidthMHz: number;
	    currentRssi: number;
	    rxRateMbps: number;
	    txRateMbps: number;
	    completedAt: string;
	    accessPoints: AccessPoint[];
	    scores24Ghz: ChannelScore[];
	    scores5Ghz: ChannelScore[];
	    scores6Ghz: ChannelScore[];
	    rec24Ghz: BandRecommendation;
	    rec5Ghz: BandRecommendation;
	    rec6Ghz: BandRecommendation;
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	        this.adapterName = source["adapterName"];
	        this.connected = source["connected"];
	        this.currentSsid = source["currentSsid"];
	        this.currentBssid = source["currentBssid"];
	        this.currentBand = source["currentBand"];
	        this.currentChannel = source["currentChannel"];
	        this.currentWidthMHz = source["currentWidthMHz"];
	        this.currentRssi = source["currentRssi"];
	        this.rxRateMbps = source["rxRateMbps"];
	        this.txRateMbps = source["txRateMbps"];
	        this.completedAt = source["completedAt"];
	        this.accessPoints = this.convertValues(source["accessPoints"], AccessPoint);
	        this.scores24Ghz = this.convertValues(source["scores24Ghz"], ChannelScore);
	        this.scores5Ghz = this.convertValues(source["scores5Ghz"], ChannelScore);
	        this.scores6Ghz = this.convertValues(source["scores6Ghz"], ChannelScore);
	        this.rec24Ghz = this.convertValues(source["rec24Ghz"], BandRecommendation);
	        this.rec5Ghz = this.convertValues(source["rec5Ghz"], BandRecommendation);
	        this.rec6Ghz = this.convertValues(source["rec6Ghz"], BandRecommendation);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

