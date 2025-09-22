export namespace models {
	
	export class User {
	    ID: number;
	    Name: string;
	    Email: string;
	    Password: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Email = source["Email"];
	        this.Password = source["Password"];
	    }
	}

}

