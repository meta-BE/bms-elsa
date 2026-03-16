export namespace app {
	
	export class DiffImportCandidateDTO {
	    filePath: string;
	    fileName: string;
	    title: string;
	    subtitle: string;
	    artist: string;
	    subartist: string;
	    destFolder: string;
	    score: number;
	    matchMethod: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffImportCandidateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filePath = source["filePath"];
	        this.fileName = source["fileName"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.artist = source["artist"];
	        this.subartist = source["subartist"];
	        this.destFolder = source["destFolder"];
	        this.score = source["score"];
	        this.matchMethod = source["matchMethod"];
	    }
	}
	export class DiffImportResultDTO {
	    success: number;
	    failed: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new DiffImportResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.failed = source["failed"];
	        this.errors = source["errors"];
	    }
	}
	export class MergeFoldersResultDTO {
	    success: boolean;
	    moved: number;
	    replaced: number;
	    skipped: number;
	    errors: number;
	    errorMsg: string;
	
	    static createFrom(source: any = {}) {
	        return new MergeFoldersResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.moved = source["moved"];
	        this.replaced = source["replaced"];
	        this.skipped = source["skipped"];
	        this.errors = source["errors"];
	        this.errorMsg = source["errorMsg"];
	    }
	}

}

export namespace dto {
	
	export class DifficultyLabelDTO {
	    tableName: string;
	    symbol: string;
	    level: string;
	
	    static createFrom(source: any = {}) {
	        return new DifficultyLabelDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tableName = source["tableName"];
	        this.symbol = source["symbol"];
	        this.level = source["level"];
	    }
	}
	export class ChartDTO {
	    md5: string;
	    sha256: string;
	    title: string;
	    subtitle?: string;
	    artist?: string;
	    subArtist?: string;
	    mode: number;
	    difficulty: number;
	    level: number;
	    minBpm: number;
	    maxBpm: number;
	    path?: string;
	    notes: number;
	    hasIrMeta: boolean;
	    lr2irTags?: string;
	    lr2irBodyUrl?: string;
	    lr2irDiffUrl?: string;
	    lr2irNotes?: string;
	    workingBodyUrl?: string;
	    workingDiffUrl?: string;
	    difficultyLabels?: DifficultyLabelDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ChartDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.md5 = source["md5"];
	        this.sha256 = source["sha256"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.artist = source["artist"];
	        this.subArtist = source["subArtist"];
	        this.mode = source["mode"];
	        this.difficulty = source["difficulty"];
	        this.level = source["level"];
	        this.minBpm = source["minBpm"];
	        this.maxBpm = source["maxBpm"];
	        this.path = source["path"];
	        this.notes = source["notes"];
	        this.hasIrMeta = source["hasIrMeta"];
	        this.lr2irTags = source["lr2irTags"];
	        this.lr2irBodyUrl = source["lr2irBodyUrl"];
	        this.lr2irDiffUrl = source["lr2irDiffUrl"];
	        this.lr2irNotes = source["lr2irNotes"];
	        this.workingBodyUrl = source["workingBodyUrl"];
	        this.workingDiffUrl = source["workingDiffUrl"];
	        this.difficultyLabels = this.convertValues(source["difficultyLabels"], DifficultyLabelDTO);
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
	export class ChartIRMetaDTO {
	    md5: string;
	    hasIrMeta: boolean;
	    lr2irTags?: string;
	    lr2irBodyUrl?: string;
	    lr2irDiffUrl?: string;
	    lr2irNotes?: string;
	    workingBodyUrl?: string;
	    workingDiffUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChartIRMetaDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.md5 = source["md5"];
	        this.hasIrMeta = source["hasIrMeta"];
	        this.lr2irTags = source["lr2irTags"];
	        this.lr2irBodyUrl = source["lr2irBodyUrl"];
	        this.lr2irDiffUrl = source["lr2irDiffUrl"];
	        this.lr2irNotes = source["lr2irNotes"];
	        this.workingBodyUrl = source["workingBodyUrl"];
	        this.workingDiffUrl = source["workingDiffUrl"];
	    }
	}
	export class ChartListItemDTO {
	    md5: string;
	    title: string;
	    subtitle?: string;
	    artist: string;
	    subArtist?: string;
	    genre: string;
	    minBpm: number;
	    maxBpm: number;
	    difficulty: number;
	    notes: number;
	    eventName?: string;
	    releaseYear?: number;
	    hasIrMeta: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChartListItemDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.md5 = source["md5"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.artist = source["artist"];
	        this.subArtist = source["subArtist"];
	        this.genre = source["genre"];
	        this.minBpm = source["minBpm"];
	        this.maxBpm = source["maxBpm"];
	        this.difficulty = source["difficulty"];
	        this.notes = source["notes"];
	        this.eventName = source["eventName"];
	        this.releaseYear = source["releaseYear"];
	        this.hasIrMeta = source["hasIrMeta"];
	    }
	}
	
	export class DifficultyTableDTO {
	    id: number;
	    url: string;
	    name: string;
	    symbol: string;
	    entryCount: number;
	    fetchedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new DifficultyTableDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.name = source["name"];
	        this.symbol = source["symbol"];
	        this.entryCount = source["entryCount"];
	        this.fetchedAt = source["fetchedAt"];
	    }
	}
	export class DifficultyTableEntryDTO {
	    md5: string;
	    level: string;
	    title: string;
	    artist: string;
	    url: string;
	    urlDiff: string;
	    status: string;
	    installedCount: number;
	
	    static createFrom(source: any = {}) {
	        return new DifficultyTableEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.md5 = source["md5"];
	        this.level = source["level"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.url = source["url"];
	        this.urlDiff = source["urlDiff"];
	        this.status = source["status"];
	        this.installedCount = source["installedCount"];
	    }
	}
	export class DifficultyTableRefreshResult {
	    tableName: string;
	    success: boolean;
	    entryCount: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DifficultyTableRefreshResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tableName = source["tableName"];
	        this.success = source["success"];
	        this.entryCount = source["entryCount"];
	        this.error = source["error"];
	    }
	}
	export class EventMappingDTO {
	    id: number;
	    urlPattern: string;
	    eventName: string;
	    releaseYear: number;
	
	    static createFrom(source: any = {}) {
	        return new EventMappingDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.urlPattern = source["urlPattern"];
	        this.eventName = source["eventName"];
	        this.releaseYear = source["releaseYear"];
	    }
	}
	export class InferWorkingURLResultDTO {
	    applied: number;
	    skipped: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new InferWorkingURLResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applied = source["applied"];
	        this.skipped = source["skipped"];
	        this.total = source["total"];
	    }
	}
	export class SongIRURLsDTO {
	    folderHash: string;
	    title: string;
	    artist: string;
	    genre: string;
	    bodyUrls: string[];
	    chartCount: number;
	    irCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SongIRURLsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderHash = source["folderHash"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.genre = source["genre"];
	        this.bodyUrls = source["bodyUrls"];
	        this.chartCount = source["chartCount"];
	        this.irCount = source["irCount"];
	    }
	}
	export class InferenceResultDTO {
	    autoSetCount: number;
	    unmatchedSongs: SongIRURLsDTO[];
	    noIRCount: number;
	
	    static createFrom(source: any = {}) {
	        return new InferenceResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoSetCount = source["autoSetCount"];
	        this.unmatchedSongs = this.convertValues(source["unmatchedSongs"], SongIRURLsDTO);
	        this.noIRCount = source["noIRCount"];
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
	export class InstallCandidateDTO {
	    folderPath: string;
	    title: string;
	    artist: string;
	    matchTypes: string[];
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new InstallCandidateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderPath = source["folderPath"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.matchTypes = source["matchTypes"];
	        this.score = source["score"];
	    }
	}
	export class RewriteRuleDTO {
	    id: number;
	    ruleType: string;
	    pattern: string;
	    replacement: string;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new RewriteRuleDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ruleType = source["ruleType"];
	        this.pattern = source["pattern"];
	        this.replacement = source["replacement"];
	        this.priority = source["priority"];
	    }
	}
	export class SongDetailDTO {
	    folderHash: string;
	    title: string;
	    artist: string;
	    genre: string;
	    eventName?: string;
	    releaseYear?: number;
	    charts: ChartDTO[];
	
	    static createFrom(source: any = {}) {
	        return new SongDetailDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderHash = source["folderHash"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.genre = source["genre"];
	        this.eventName = source["eventName"];
	        this.releaseYear = source["releaseYear"];
	        this.charts = this.convertValues(source["charts"], ChartDTO);
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
	
	export class SongRowDTO {
	    folderHash: string;
	    title: string;
	    artist: string;
	    genre: string;
	    minBpm: number;
	    maxBpm: number;
	    eventName?: string;
	    releaseYear?: number;
	    hasIrMeta: boolean;
	    chartCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SongRowDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderHash = source["folderHash"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.genre = source["genre"];
	        this.minBpm = source["minBpm"];
	        this.maxBpm = source["maxBpm"];
	        this.eventName = source["eventName"];
	        this.releaseYear = source["releaseYear"];
	        this.hasIrMeta = source["hasIrMeta"];
	        this.chartCount = source["chartCount"];
	    }
	}
	export class SongListDTO {
	    songs: SongRowDTO[];
	    totalCount: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new SongListDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.songs = this.convertValues(source["songs"], SongRowDTO);
	        this.totalCount = source["totalCount"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
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
	
	export class Config {
	    songdataDBPath: string;
	    fileLog: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.songdataDBPath = source["songdataDBPath"];
	        this.fileLog = source["fileLog"];
	    }
	}

}

export namespace similarity {
	
	export class ScoreResult {
	    WAV: number;
	    Title: number;
	    Artist: number;
	    Genre: number;
	    BPM: number;
	    Total: number;
	
	    static createFrom(source: any = {}) {
	        return new ScoreResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.WAV = source["WAV"];
	        this.Title = source["Title"];
	        this.Artist = source["Artist"];
	        this.Genre = source["Genre"];
	        this.BPM = source["BPM"];
	        this.Total = source["Total"];
	    }
	}
	export class DuplicateMember {
	    FolderHash: string;
	    Title: string;
	    Artist: string;
	    Genre: string;
	    MinBPM: number;
	    MaxBPM: number;
	    ChartCount: number;
	    Path: string;
	    Scores: ScoreResult;
	
	    static createFrom(source: any = {}) {
	        return new DuplicateMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FolderHash = source["FolderHash"];
	        this.Title = source["Title"];
	        this.Artist = source["Artist"];
	        this.Genre = source["Genre"];
	        this.MinBPM = source["MinBPM"];
	        this.MaxBPM = source["MaxBPM"];
	        this.ChartCount = source["ChartCount"];
	        this.Path = source["Path"];
	        this.Scores = this.convertValues(source["Scores"], ScoreResult);
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
	export class DuplicateGroup {
	    ID: number;
	    Members: DuplicateMember[];
	    Score: number;
	
	    static createFrom(source: any = {}) {
	        return new DuplicateGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Members = this.convertValues(source["Members"], DuplicateMember);
	        this.Score = source["Score"];
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

