"use strict";

var _interopRequire = function (obj) { return obj && obj.__esModule ? obj["default"] : obj; };

var _slicedToArray = function (arr, i) { if (Array.isArray(arr)) { return arr; } else if (Symbol.iterator in Object(arr)) { var _arr = []; for (var _iterator = arr[Symbol.iterator](), _step; !(_step = _iterator.next()).done;) { _arr.push(_step.value); if (i && _arr.length === i) break; } return _arr; } else { throw new TypeError("Invalid attempt to destructure non-iterable instance"); } };

var _createClass = (function () { function defineProperties(target, props) { for (var key in props) { var prop = props[key]; prop.configurable = true; if (prop.value) prop.writable = true; } Object.defineProperties(target, props); } return function (Constructor, protoProps, staticProps) { if (protoProps) defineProperties(Constructor.prototype, protoProps); if (staticProps) defineProperties(Constructor, staticProps); return Constructor; }; })();

var _classCallCheck = function (instance, Constructor) { if (!(instance instanceof Constructor)) { throw new TypeError("Cannot call a class as a function"); } };

var $ = _interopRequire(require("jquery"));

require("jquery-ui/autocomplete");

var ko = _interopRequire(require("knockout"));

var uuid = require("node-uuid").v4;

var API_BASE_URL = "" + location.origin + "/";
var API_REPOS_URL = "" + API_BASE_URL + "/secure/repos";
var API_LOGIN_URL = "" + API_BASE_URL + "/login";
var API_LOGOUT_URL = "" + API_BASE_URL + "/logout";

// keep knockout-jqAutocomplete happy
global.jQuery = $;
global.ko = ko;

require("knockout-jqAutocomplete/src/knockout-jqAutocomplete");

var Model = (function () {
	function Model() {
		var _this = this;

		_classCallCheck(this, Model);

		this.error = ko.observable();
		this.ready = ko.observable(false);
		this.loggedIn = ko.observable(false);
		this.repos = ko.observableArray();
		this._selectedRepo = ko.observable();
		this.selectedRepo = ko.computed(function () {
			var repo = _this._selectedRepo();
			if (repo && repo.Owner && repo.Name) {
				return repo;
			}
		});
		this.selectedRepoDetails = ko.observable();
		this.viewSelectedRepoDetails = ko.observable();
		this.userEnteredFullName = ko.observable();
		this.loadingRepoDetails = ko.computed(function () {
			return _this.selectedRepo() && !_this.selectedRepoDetails();
		});
		this.resetEnabled = ko.computed(function () {
			return _this.selectedRepo() && _this.userEnteredFullName() === "" + _this.selectedRepo().Owner + "/" + _this.selectedRepo().Name;
		});

		this.selectedRepoSummary = ko.computed(function () {
			if (!_this.selectedRepoDetails()) {
				return;
			}
			return _this.selectedRepoDetails().BranchList.reduce(function (summary, branch) {
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
		this.selectedRepoHasChanges = ko.computed(function () {
			var summary = _this.selectedRepoSummary();
			return summary && summary.created + summary.deleted + summary.reset > 0;
		});
		this.selectedRepoInSync = ko.computed(function () {
			return _this.selectedRepoDetails() && !_this.selectedRepoHasChanges();
		});

		this.selectedRepo.subscribe(function () {
			return _this.loadSelectedRepoDetails();
		});

		this.refreshRepos().then(function (d) {
			return _this.ready(true);
		}, function (e) {
			return _this.ready(true);
		});
	}

	_createClass(Model, {
		_get: {
			value: function _get(url) {
				var _this = this;

				return $.ajax({
					type: "GET",
					url: url,
					dataType: "JSON",
					xhrFields: {
						withCredentials: true
					}
				}).then(function (d) {
					return (_this.loggedIn(true), d);
				}, function (e) {
					return _this._handleAjaxError(e);
				});
			}
		},
		_post: {
			value: function _post(url) {
				var _this = this;

				var _document$cookie$match = document.cookie.match(/session=([^;]+)/);

				var _document$cookie$match2 = _slicedToArray(_document$cookie$match, 2);

				var csrfToken = _document$cookie$match2[1];

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
				}).then(function (d) {
					return (_this.loggedIn(true), d);
				}, function (e) {
					return _this._handleAjaxError(e);
				});
			}
		},
		_handleAjaxError: {
			value: function _handleAjaxError(e) {
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
		},
		toggleSession: {
			value: function toggleSession() {
				if (this.loggedIn()) {
					location = API_LOGOUT_URL;
				} else {
					location = API_LOGIN_URL;
				}
			}
		},
		refreshRepos: {
			value: function refreshRepos() {
				var _this = this;

				return this._get(API_REPOS_URL).then(function (repos) {
					_this.repos(repos.map(function (repo) {
						return (repo.FullName = repo.Owner + "/" + repo.Name, repo);
					}));
				});
			}
		},
		_getSelectedRepoUrl: {
			value: function _getSelectedRepoUrl() {
				return "" + API_REPOS_URL + "/" + this.selectedRepo().Owner + "/" + this.selectedRepo().Name;
			}
		},
		toggleSelectedRepoDetails: {
			value: function toggleSelectedRepoDetails() {
				this.viewSelectedRepoDetails(!this.viewSelectedRepoDetails());
			}
		},
		loadSelectedRepoDetails: {
			value: function loadSelectedRepoDetails() {
				var _this = this;

				this.selectedRepoDetails(null);
				if (this.selectedRepo()) {
					this._get(this._getSelectedRepoUrl()).then(function (repoDetails) {
						return _this._setSelectedRepoDetails(repoDetails);
					});
				}
			}
		},
		_setSelectedRepoDetails: {
			value: function _setSelectedRepoDetails(repoDetails) {
				repoDetails.BranchList = Object.keys(repoDetails.Branches).map(function (name) {
					return {
						Name: name,
						SHA: repoDetails.Branches[name].SHA,
						ParentSHA: repoDetails.Branches[name].ParentSHA
					};
				});
				this.selectedRepoDetails(repoDetails);
			}
		},
		branchClassName: {
			value: function branchClassName(branch) {
				return branch.SHA === branch.ParentSHA ? "success" : !branch.SHA && branch.ParentSHA ? "info" : branch.SHA && !branch.ParentSHA ? "danger" : "warning";
			}
		},
		reset: {
			value: function reset() {
				var _this = this;

				if (!this.resetEnabled()) {
					throw new Error("That wasn't supported to happen.");
				}
				this.userEnteredFullName(null);
				this.selectedRepoDetails(null);
				this._post("" + this._getSelectedRepoUrl() + "/resets").then(function (repoDetails) {
					return _this._setSelectedRepoDetails(repoDetails);
				});
			}
		}
	});

	return Model;
})();

ko.applyBindings(new Model());
//# sourceMappingURL=app.js.map