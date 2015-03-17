import $ from "jquery";
import "jquery-ui/autocomplete";
import ko from "knockout";
import {v4 as uuid} from "node-uuid";

const API_BASE_URL = `${location.origin}`;
const API_REPOS_URL = `${API_BASE_URL}/secure/repos`;
const API_LOGIN_URL = `${API_BASE_URL}/login`;
const API_LOGOUT_URL = `${API_BASE_URL}/logout`;

// keep knockout-jqAutocomplete happy
global.jQuery = $;
global.ko = ko;
import "knockout-jqAutocomplete/src/knockout-jqAutocomplete"

class Model {
	constructor() {
		this.error = ko.observable();
		this.ready = ko.observable(false);
		this.loggedIn = ko.observable(false);
		this.repos = ko.observableArray();
		this._selectedRepo = ko.observable();
		this.selectedRepo = ko.computed(() => {
			const repo = this._selectedRepo();
			if (repo && repo.Owner && repo.Name) {
				return repo;
			}
		});
		this.selectedRepoDetails = ko.observable();
		this.viewSelectedRepoDetails = ko.observable();
		this.userEnteredFullName = ko.observable();
		this.loadingRepoDetails = ko.computed(() => this.selectedRepo() && !this.selectedRepoDetails());
		this.resetEnabled = ko.computed(() => this.selectedRepo() &&
			this.userEnteredFullName() === `${this.selectedRepo().Owner}/${this.selectedRepo().Name}`);

		this.selectedRepoSummary = ko.computed(() => {
			if (!this.selectedRepoDetails()) {
				return;
			}
			return this.selectedRepoDetails().BranchList.reduce((summary, branch) => {
				if (branch.SHA === branch.ParentSHA) {
					summary.unchanged++;
				} else if (!branch.SHA && branch.ParentSHA) {
					summary.created++;
				} else if (branch.SHA && !branch.ParentSHA) {
					summary.deleted++;
				} else {
					summary.reset++;
				}
				return summary;
			}, {
				unchanged: 0,
				created: 0,
				reset: 0,
				deleted: 0
			});
		});
		this.selectedRepoHasChanges = ko.computed(() => {
			const summary = this.selectedRepoSummary();
			return summary && (summary.created + summary.deleted + summary.reset > 0);
		});
		this.selectedRepoInSync = ko.computed(() =>
			this.selectedRepoDetails() && !this.selectedRepoHasChanges());

		this.selectedRepo.subscribe(() => this.loadSelectedRepoDetails());

		this.refreshRepos()
			.then(d => this.ready(true), e => this.ready(true));
	}

	_get(url) {
		return $.ajax({
				type: "GET",
				url: url,
				dataType: "JSON",
				xhrFields: {
				  withCredentials: true
			   }
			})
			.then(d => (this.loggedIn(true), d), e => this._handleAjaxError(e));
	}

	_post(url) {
		const [,csrfToken] = document.cookie.match(/session=([^;]+)/);
		return $.ajax({
				type: "POST",
				url: url,
				dataType: "JSON",
				headers: {
					"X-Csrf-Token": csrfToken
				},
				xhrFields: {
				  withCredentials: true
			   }
			})
			.then(d => (this.loggedIn(true), d), e => this._handleAjaxError(e));
	}

	_handleAjaxError(e) {
		switch (e.status) {
			case 0:
			case 401:
				this.loggedIn(false);
				break;
			case 500:
			default:
				this.error("it's not you it's me, we've grown apart, or it might be you, I'm not sure...");
				break;
		}
	}

	toggleSession() {
		if (this.loggedIn()) {
			location = API_LOGOUT_URL;
		} else {
			location = API_LOGIN_URL;
		}
	}

	refreshRepos() {
		return this._get(API_REPOS_URL)
			.then(repos => {
				this.repos(repos.map(repo => (repo.FullName = repo.Owner + '/' + repo.Name, repo)));
			});
	}

	_getSelectedRepoUrl() {
		return `${API_REPOS_URL}/${this.selectedRepo().Owner}/${this.selectedRepo().Name}`;
	}

	toggleSelectedRepoDetails() {
		this.viewSelectedRepoDetails(!this.viewSelectedRepoDetails());
	}

	loadSelectedRepoDetails() {
		this.selectedRepoDetails(null);
		if (this.selectedRepo()) {
			this._get(this._getSelectedRepoUrl())
				.then(repoDetails => this._setSelectedRepoDetails(repoDetails));
		}
	}

	_setSelectedRepoDetails(repoDetails) {
		repoDetails.BranchList =
			Object.keys(repoDetails.Branches)
				.map(name => ({
					Name: name,
					SHA: repoDetails.Branches[name].SHA,
					ParentSHA: repoDetails.Branches[name].ParentSHA
				}));
		this.selectedRepoDetails(repoDetails);
	}

	branchClassName(branch) {
		return (branch.SHA === branch.ParentSHA) ? 'success' :
			   (!branch.SHA && branch.ParentSHA) ? 'info' :
			   (branch.SHA && !branch.ParentSHA) ? 'danger' :
			   'warning';
	}

	reset() {
		if (!this.resetEnabled()) {
			throw new Error("That wasn't supported to happen.");
		}
		this.userEnteredFullName(null);
		this.selectedRepoDetails(null);
		this._post(`${this._getSelectedRepoUrl()}/resets`)
			.then(repoDetails => this._setSelectedRepoDetails(repoDetails));
	}
}

ko.applyBindings(new Model());
