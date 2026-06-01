export namespace main {
	
	export class Config {
	    path: string;
	    httpPort: string;
	    friendlyName: string;
	    ifname: string;
	    allowedIps: string;
	    deviceIcon: string;
	    ffprobeCachePath: string;
	    forceTranscodeTo: string;
	    ignoreHidden: boolean;
	    ignoreUnreadable: boolean;
	    noProbe: boolean;
	    noTranscode: boolean;
	    autoStartServer: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.httpPort = source["httpPort"];
	        this.friendlyName = source["friendlyName"];
	        this.ifname = source["ifname"];
	        this.allowedIps = source["allowedIps"];
	        this.deviceIcon = source["deviceIcon"];
	        this.ffprobeCachePath = source["ffprobeCachePath"];
	        this.forceTranscodeTo = source["forceTranscodeTo"];
	        this.ignoreHidden = source["ignoreHidden"];
	        this.ignoreUnreadable = source["ignoreUnreadable"];
	        this.noProbe = source["noProbe"];
	        this.noTranscode = source["noTranscode"];
	        this.autoStartServer = source["autoStartServer"];
	    }
	}
	export class Tools {
	    ffmpeg: boolean;
	    ffprobe: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tools(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ffmpeg = source["ffmpeg"];
	        this.ffprobe = source["ffprobe"];
	    }
	}

}

