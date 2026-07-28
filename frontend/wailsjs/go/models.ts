export namespace main {
	
	export class RepoSource {
	    name: string;
	    kind: string;
	    repo: string;
	    token?: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.repo = source["repo"];
	        this.token = source["token"];
	        this.enabled = source["enabled"];
	    }
	}
	export class AppConfig {
	    steamPath: string;
	    theme: string;
	    lastZipDir: string;
	    repoSources: RepoSource[];
	    autoDetect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.steamPath = source["steamPath"];
	        this.theme = source["theme"];
	        this.lastZipDir = source["lastZipDir"];
	        this.repoSources = this.convertValues(source["repoSources"], RepoSource);
	        this.autoDetect = source["autoDetect"];
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
	export class DLCInfo {
	    appID: string;
	    name: string;
	    hasKey: boolean;
	    decryptionKey: string;
	    isInstalled: boolean;
	    manifestID: string;
	    fileSize: number;
	
	    static createFrom(source: any = {}) {
	        return new DLCInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appID = source["appID"];
	        this.name = source["name"];
	        this.hasKey = source["hasKey"];
	        this.decryptionKey = source["decryptionKey"];
	        this.isInstalled = source["isInstalled"];
	        this.manifestID = source["manifestID"];
	        this.fileSize = source["fileSize"];
	    }
	}
	export class DeployedEntry {
	    fileName: string;
	    mainAppID: string;
	    appIDs: string[];
	    isExternal: boolean;
	    inHistory: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeployedEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.mainAppID = source["mainAppID"];
	        this.appIDs = source["appIDs"];
	        this.isExternal = source["isExternal"];
	        this.inHistory = source["inHistory"];
	    }
	}
	export class DepotInfo {
	    depotID: string;
	    decryptionKey: string;
	    manifestID: string;
	    fileSize: number;
	
	    static createFrom(source: any = {}) {
	        return new DepotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.depotID = source["depotID"];
	        this.decryptionKey = source["decryptionKey"];
	        this.manifestID = source["manifestID"];
	        this.fileSize = source["fileSize"];
	    }
	}
	export class DetectorResult {
	    name: string;
	    status: string;
	    available: boolean;
	    message: string;
	    missingFiles: string[];
	    checkedPath: string;
	
	    static createFrom(source: any = {}) {
	        return new DetectorResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.available = source["available"];
	        this.message = source["message"];
	        this.missingFiles = source["missingFiles"];
	        this.checkedPath = source["checkedPath"];
	    }
	}
	export class GameDetail {
	    appID: string;
	    name: string;
	    headerImage: string;
	    description: string;
	    developers: string[];
	    publishers: string[];
	    releaseDate: string;
	    screenshots: string[];
	    dlcIDs: string[];
	
	    static createFrom(source: any = {}) {
	        return new GameDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appID = source["appID"];
	        this.name = source["name"];
	        this.headerImage = source["headerImage"];
	        this.description = source["description"];
	        this.developers = source["developers"];
	        this.publishers = source["publishers"];
	        this.releaseDate = source["releaseDate"];
	        this.screenshots = source["screenshots"];
	        this.dlcIDs = source["dlcIDs"];
	    }
	}
	export class GamePackage {
	    mainAppID: string;
	    mainKey: string;
	    gameName: string;
	    depots: DepotInfo[];
	    dlcs: DLCInfo[];
	    luaContent: string;
	    manifestFiles: string[];
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new GamePackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mainAppID = source["mainAppID"];
	        this.mainKey = source["mainKey"];
	        this.gameName = source["gameName"];
	        this.depots = this.convertValues(source["depots"], DepotInfo);
	        this.dlcs = this.convertValues(source["dlcs"], DLCInfo);
	        this.luaContent = source["luaContent"];
	        this.manifestFiles = source["manifestFiles"];
	        this.source = source["source"];
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
	export class GameRecord {
	    mainAppID: string;
	    gameName: string;
	    dlcCount: number;
	    installedIDs: string[];
	    installedAt: string;
	    luaFileName: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new GameRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mainAppID = source["mainAppID"];
	        this.gameName = source["gameName"];
	        this.dlcCount = source["dlcCount"];
	        this.installedIDs = source["installedIDs"];
	        this.installedAt = source["installedAt"];
	        this.luaFileName = source["luaFileName"];
	        this.source = source["source"];
	    }
	}
	export class GameSearchResult {
	    appID: string;
	    name: string;
	    headerImage: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appID = source["appID"];
	        this.name = source["name"];
	        this.headerImage = source["headerImage"];
	        this.available = source["available"];
	    }
	}
	export class MSiteStats {
	    username: string;
	    dailyUsage: number;
	    dailyLimit: number;
	    canMakeRequests: boolean;
	    expiresAt: string;
	    expiringSoon: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MSiteStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.dailyUsage = source["dailyUsage"];
	        this.dailyLimit = source["dailyLimit"];
	        this.canMakeRequests = source["canMakeRequests"];
	        this.expiresAt = source["expiresAt"];
	        this.expiringSoon = source["expiringSoon"];
	    }
	}
	export class OperationResult {
	    success: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new OperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}

}

