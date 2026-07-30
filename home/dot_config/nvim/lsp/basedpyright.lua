return {
	cmd = { 'basedpyright-langserver', '--stdio' },
	filetypes = { 'python' },
	root_markers = {
		'pyproject.toml', 'pyrightconfig.json',
		'setup.py', 'setup.cfg',
		'requirements.txt', 'Pipfile', '.git',
	},
	settings = {
		basedpyright = {
			analysis = {
				autoSearchPaths = true,
				diagnosticMode = 'openFilesOnly',
				useLibraryCodeForTypes = true,
			},
		},
	},
	before_init = function(_, config)
		local venv = config.root_dir .. '/.venv/bin/python'
		if vim.uv.fs_stat(venv) then
			config.settings.python = { pythonPath = venv }
		elseif vim.env.VIRTUAL_ENV then
			config.settings.python = {
				pythonPath = vim.env.VIRTUAL_ENV .. '/bin/python',
			}
		end
	end,
}
