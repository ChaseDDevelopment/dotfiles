return {
	cmd = { 'yaml-language-server', '--stdio' },
	filetypes = { 'yaml', 'yaml.docker-compose' },
	root_markers = { '.git' },
	settings = {
		yaml = {
			schemaStore = { enable = false, url = '' },
			schemas = require('schemastore').yaml.schemas(),
		},
	},
}
