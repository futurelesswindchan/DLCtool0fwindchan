export namespace main {
	
	export class RepoSource {
	    name: string;
	    type: string;
	    url: string;
	    mirror: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.mirror = source["mirror"];
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
	export class GamePackage {
	    mainAppID: string;
	    gameName: string;
	    depots: DepotInfo[];
	    dlcs: DLCInfo[];
	    luaContent: string;
	    manifestFiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new GamePackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mainAppID = source["mainAppID"];
	        this.gameName = source["gameName"];
	        this.depots = this.convertValues(source["depots"], DepotInfo);
	        this.dlcs = this.convertValues(source["dlcs"], DLCInfo);
	        this.luaContent = source["luaContent"];
	        this.manifestFiles = source["manifestFiles"];
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

