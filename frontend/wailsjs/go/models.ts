export namespace models {
	
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
	    folder_name: string;
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
	        this.folder_name = source["folder_name"];
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
	export class TaskRecord {
	    ID: number;
	    UserID: string;
	    User: User;
	    TaskID: string;
	    Task: Task;
	    FolderName: string;
	    // Go type: time
	    StartedAt: any;
	    // Go type: time
	    EndedAt?: any;
	    Duration?: number;
	
	    static createFrom(source: any = {}) {
	        return new TaskRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.UserID = source["UserID"];
	        this.User = this.convertValues(source["User"], User);
	        this.TaskID = source["TaskID"];
	        this.Task = this.convertValues(source["Task"], Task);
	        this.FolderName = source["FolderName"];
	        this.StartedAt = this.convertValues(source["StartedAt"], null);
	        this.EndedAt = this.convertValues(source["EndedAt"], null);
	        this.Duration = source["Duration"];
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
	

}

export namespace services {
	
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

}

