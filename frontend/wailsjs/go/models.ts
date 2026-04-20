export namespace models {
	
	export class Screenshot {
	    ID: number;
	    TaskRecordID: number;
	    TaskRecord: TaskRecord;
	    TaskID: string;
	    Task: Task;
	    Memo: string;
	    Notes: string;
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
	        this.Memo = source["Memo"];
	        this.Notes = source["Notes"];
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
	export class Client {
	    ID: string;
	    Fullname: string;
	    CompanyName: string;
	    Tasks: Task[];
	
	    static createFrom(source: any = {}) {
	        return new Client(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Fullname = source["Fullname"];
	        this.CompanyName = source["CompanyName"];
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
	    ProjectName: string;
	    ClientID: string;
	    Client: Client;
	    TaskRecord: TaskRecord[];
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.TaskID = source["TaskID"];
	        this.Title = source["Title"];
	        this.ProjectName = source["ProjectName"];
	        this.ClientID = source["ClientID"];
	        this.Client = this.convertValues(source["Client"], Client);
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
	    Padagan: boolean;
	    // Go type: time
	    EndedAt?: any;
	    Duration: number;
	    // Go type: time
	    SyncedAt?: any;
	    Productivity?: string;
	    SyncStatus: string;
	    Gap: number;
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
	        this.Padagan = source["Padagan"];
	        this.EndedAt = this.convertValues(source["EndedAt"], null);
	        this.Duration = source["Duration"];
	        this.SyncedAt = this.convertValues(source["SyncedAt"], null);
	        this.Productivity = source["Productivity"];
	        this.SyncStatus = source["SyncStatus"];
	        this.Gap = source["Gap"];
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
	export class User {
	    ID: string;
	    Name: string;
	    FirstName: string;
	    LastName: string;
	    MiddleName: string;
	    Position: string;
	    Birthdate: string;
	    Phone: string;
	    Address: string;
	    Photo: string;
	    Email: string;
	    FESID: string;
	    SupervisorId: string;
	    EmployeeId: string;
	    EmploymentId: string;
	    Attendance: Attendance[];
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.FirstName = source["FirstName"];
	        this.LastName = source["LastName"];
	        this.MiddleName = source["MiddleName"];
	        this.Position = source["Position"];
	        this.Birthdate = source["Birthdate"];
	        this.Phone = source["Phone"];
	        this.Address = source["Address"];
	        this.Photo = source["Photo"];
	        this.Email = source["Email"];
	        this.FESID = source["FESID"];
	        this.SupervisorId = source["SupervisorId"];
	        this.EmployeeId = source["EmployeeId"];
	        this.EmploymentId = source["EmploymentId"];
	        this.Attendance = this.convertValues(source["Attendance"], Attendance);
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
	    ClientTime: number;
	    TotalGap: number;
	    // Go type: time
	    TimeIn: any;
	    // Go type: time
	    TimeOut?: any;
	    LoginLogoutID?: string;
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
	        this.ClientTime = source["ClientTime"];
	        this.TotalGap = source["TotalGap"];
	        this.TimeIn = this.convertValues(source["TimeIn"], null);
	        this.TimeOut = this.convertValues(source["TimeOut"], null);
	        this.LoginLogoutID = source["LoginLogoutID"];
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
	export class ActivityLogRecord {
	    attendance: Attendance;
	    records: TaskRecord[];
	
	    static createFrom(source: any = {}) {
	        return new ActivityLogRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attendance = this.convertValues(source["attendance"], Attendance);
	        this.records = this.convertValues(source["records"], TaskRecord);
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
	    name: string;
	    position: string;
	    birthdate: string;
	    phone: string;
	    address: string;
	    photo: string;
	    fesid: string;
	    supervisor_id: string;
	    employee_id: string;
	    employment_id: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.localId = source["localId"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.position = source["position"];
	        this.birthdate = source["birthdate"];
	        this.phone = source["phone"];
	        this.address = source["address"];
	        this.photo = source["photo"];
	        this.fesid = source["fesid"];
	        this.supervisor_id = source["supervisor_id"];
	        this.employee_id = source["employee_id"];
	        this.employment_id = source["employment_id"];
	    }
	}
	
	export class ExpAttendance {
	    id: number;
	    sync_status: string;
	
	    static createFrom(source: any = {}) {
	        return new ExpAttendance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sync_status = source["sync_status"];
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
	    duration: number;
	    padagan: boolean;
	    sync_status: string;
	
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
	        this.padagan = source["padagan"];
	        this.sync_status = source["sync_status"];
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
	export class OfflineLoginProps {
	    AuthUser: AuthUser;
	    Attendance: Attendance;
	
	    static createFrom(source: any = {}) {
	        return new OfflineLoginProps(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.AuthUser = this.convertValues(source["AuthUser"], AuthUser);
	        this.Attendance = this.convertValues(source["Attendance"], Attendance);
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

export namespace options {
	
	export class SecondInstanceData {
	    Args: string[];
	    WorkingDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new SecondInstanceData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Args = source["Args"];
	        this.WorkingDirectory = source["WorkingDirectory"];
	    }
	}

}

export namespace types {
	
	export class TaskSummary {
	    TaskID: string;
	    Padagan: boolean;
	    TotalDuration: number;
	    TaskName: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.TaskID = source["TaskID"];
	        this.Padagan = source["Padagan"];
	        this.TotalDuration = source["TotalDuration"];
	        this.TaskName = source["TaskName"];
	    }
	}

}

