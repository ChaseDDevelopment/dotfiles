-- Plugin lifecycle (PackChanged auto-build + launch update check) lives in
-- core/pack. setup() registers the PackChanged hook and MUST run before
-- init.lua's vim.pack.add (this module is required just above it), so
-- install-time PackChanged events are caught.
local pack = require('core.pack')
pack.setup()

-- Check for plugin updates in the background shortly after the UI is ready.
vim.api.nvim_create_autocmd('UIEnter', {
	once = true,
	callback = function()
		vim.defer_fn(pack.check_updates, 5000)
	end,
})

-- Land in the snacks explorer instead of an empty buffer: open it for
-- `nvim <dir>` and for a bare `nvim` (no file args). The latter makes
-- Supacode's "$EDITOR" button — which runs plain `nvim` in the worktree —
-- open the project. Files and piped stdin are left untouched.
vim.api.nvim_create_autocmd('UIEnter', {
	callback = function()
		local ui = vim.api.nvim_list_uis()[1]
		if ui and ui.stdout_tty and not ui.stdin_tty then
			return -- input is piped (e.g. `echo x | nvim`)
		end

		local bufname = vim.api.nvim_buf_get_name(0)
		local dir
		if bufname ~= '' and vim.fn.isdirectory(bufname) == 1 then
			dir = bufname -- nvim <dir>
		elseif bufname == '' and vim.fn.argc(-1) == 0 then
			dir = vim.fn.getcwd() -- bare nvim
		else
			return -- files passed → open them as-is
		end

		vim.fn.chdir(dir)
		vim.api.nvim_buf_delete(0, { force = true })
		Snacks.picker.explorer({ cwd = dir })
	end,
})

-- LSP keymaps: activate when a language server attaches to a buffer
-- Built-in 0.11 defaults: grn=rename, grr=references, gri=implementation,
-- grt=type_definition, gra=code_action, gO=document_symbol, K=hover, <C-s>=signature_help
vim.api.nvim_create_autocmd('LspAttach', {
	callback = function(ev)
		vim.keymap.set('n', 'gd', vim.lsp.buf.definition, { buffer = ev.buf, desc = 'Go to definition' })
		vim.keymap.set('n', '<leader>cr', vim.lsp.buf.rename, { buffer = ev.buf, desc = 'Rename symbol' })
		vim.keymap.set('n', '<leader>ca', vim.lsp.buf.code_action, { buffer = ev.buf, desc = 'Code action' })
		vim.keymap.set('n', '<leader>cs', vim.lsp.buf.signature_help, { buffer = ev.buf, desc = 'Signature help' })

		if ev.data and ev.data.client_id then
			local client = vim.lsp.get_client_by_id(ev.data.client_id)
			if client and client:supports_method('textDocument/inlayHint') then
				vim.lsp.inlay_hint.enable(true, { bufnr = ev.buf })
			end
		end
	end,
})

-- Ansible: map the canonical playbook/role layout to yaml.ansible so the
-- ansible LSP (not yamlls) attaches. All other yaml stays plain yaml.
vim.filetype.add({
	pattern = {
		['.*/playbooks/.*%.ya?ml'] = 'yaml.ansible',
		['.*/roles/.*/tasks/.*%.ya?ml'] = 'yaml.ansible',
		['.*/roles/.*/handlers/.*%.ya?ml'] = 'yaml.ansible',
		['.*/tasks/.*%.ya?ml'] = 'yaml.ansible',
		['.*/handlers/.*%.ya?ml'] = 'yaml.ansible',
		['.*/group_vars/.*%.ya?ml'] = 'yaml.ansible',
		['.*/host_vars/.*%.ya?ml'] = 'yaml.ansible',
		['.*/playbook%.ya?ml'] = 'yaml.ansible',
		['.*/site%.ya?ml'] = 'yaml.ansible',
	},
})

-- systemd units: Neovim only auto-detects these under canonical systemd
-- paths, so a unit edited anywhere else gets no filetype (hence no
-- highlighting). Map the unit extensions authoritatively — they're all
-- systemd-specific; the shipped syntax/systemd.vim then highlights them.
vim.filetype.add({
	extension = {
		service = 'systemd',
		socket = 'systemd',
		device = 'systemd',
		mount = 'systemd',
		automount = 'systemd',
		swap = 'systemd',
		target = 'systemd',
		path = 'systemd',
		timer = 'systemd',
		slice = 'systemd',
		scope = 'systemd',
		netdev = 'systemd',
		network = 'systemd',
		link = 'systemd',
		nspawn = 'systemd',
		busname = 'systemd',
		dnssd = 'systemd',
	},
})
