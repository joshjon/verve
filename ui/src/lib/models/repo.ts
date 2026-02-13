export interface Repo {
	id: string;
	owner: string;
	name: string;
	full_name: string;
	created_at: string;
}

export interface GitHubRepo {
	full_name: string;
	owner_login: string;
	name: string;
	description: string;
	private: boolean;
	html_url: string;
}
