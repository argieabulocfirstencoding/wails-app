export namespace models {
	
	export class Screenshot {
	    ID: number;
	    TaskRecordID: number;
	    TaskRecord: TaskRecord;
	    TaskID: string;
	    Task: Task;
	    FilePath: string;
	    CloudURL?: string;
	    // Go type: time
	    TakenAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Screenshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.TaskRecordID = source["TaskRecordID"];
	        this.TaskRecord = this.convertValues(source["TaskRecord"], TaskRecord);
	        this.TaskID = source["TaskID"];
	        this.Task = this.convertValues(source["Task"], Task);
	        this.FilePath = source["FilePath"];
	        this.CloudURL = source["CloudURL"];
	        this.TakenAt = this.convertValues(source["TakenAt"], null);
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
	export class TaskRecord {
	    ID: number;
	    UserID: string;
	    User: User;
	    AttendanceID: number;
	    Attendance: Attendance;
	    TaskID: string;
	    Task: Task;
	    // Go type: time
	    StartedAt: any;
	    // Go type: time
	    EndedAt?: any;
	    Duration?: number;
	    // Go type: time
	    SyncedAt?: any;
	    SyncStatus: string;
	    Screenshots: Screenshot[];
	
	    static createFrom(source: any = {}) {
	        return new TaskRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.UserID = source["UserID"];
	        this.User = this.convertValues(source["User"], User);
	        this.AttendanceID = source["AttendanceID"];
	        this.Attendance = this.convertValues(source["Attendance"], Attendance);
	        this.TaskID = source["TaskID"];
	        this.Task = this.convertValues(source["Task"], Task);
	        this.StartedAt = this.convertValues(source["StartedAt"], null);
	        this.EndedAt = this.convertValues(source["EndedAt"], null);
	        this.Duration = source["Duration"];
	        this.SyncedAt = this.convertValues(source["SyncedAt"], null);
	        this.SyncStatus = source["SyncStatus"];
	        this.Screenshots = this.convertValues(source["Screenshots"], Screenshot);
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
	export class Task {
	    ID: number;
	    TaskID: string;
	    Title: string;
	    Client: string;
	    UserID: string;
	    User: User;
	    TaskRecord: TaskRecord[];
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.TaskID = source["TaskID"];
	        this.Title = source["Title"];
	        this.Client = source["Client"];
	        this.UserID = source["UserID"];
	        this.User = this.convertValues(source["User"], User);
	        this.TaskRecord = this.convertValues(source["TaskRecord"], TaskRecord);
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
	export class User {
	    ID: string;
	    Email: string;
	    Tasks: Task[];
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Email = source["Email"];
	        this.Tasks = this.convertValues(source["Tasks"], Task);
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
	export class Attendance {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    UserID: string;
	    User: User;
	    // Go type: time
	    TimeIn: any;
	    // Go type: time
	    TimeOut?: any;
	    // Go type: time
	    SyncedAt?: any;
	    SyncStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new Attendance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.UserID = source["UserID"];
	        this.User = this.convertValues(source["User"], User);
	        this.TimeIn = this.convertValues(source["TimeIn"], null);
	        this.TimeOut = this.convertValues(source["TimeOut"], null);
	        this.SyncedAt = this.convertValues(source["SyncedAt"], null);
	        this.SyncStatus = source["SyncStatus"];
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
	export class AuthUser {
	    localId: string;
	    email: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.localId = source["localId"];
	        this.email = source["email"];
	    }
	}
	export class ExpTask {
	    taskId: string;
	    title: string;
	    client: string;
	
	    static createFrom(source: any = {}) {
	        return new ExpTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.title = source["title"];
	        this.client = source["client"];
	    }
	}
	export class ExpTaskRecord {
	    id: number;
	    task_id: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    ended_at?: any;
	    duration?: number;
	
	    static createFrom(source: any = {}) {
	        return new ExpTaskRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.task_id = source["task_id"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.ended_at = this.convertValues(source["ended_at"], null);
	        this.duration = source["duration"];
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
	
	
	
	export class TaskRecordWithScreenshots {
	    record?: ExpTaskRecord;
	    // Go type: time
	    takenAt: any;
	    screenshots: string[];
	
	    static createFrom(source: any = {}) {
	        return new TaskRecordWithScreenshots(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.record = this.convertValues(source["record"], ExpTaskRecord);
	        this.takenAt = this.convertValues(source["takenAt"], null);
	        this.screenshots = source["screenshots"];
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

