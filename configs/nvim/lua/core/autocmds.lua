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

-- Replace netrw: open snacks explorer when nvim is launched with a directory
vim.api.nvim_create_autocmd('UIEnter', {
	callback = function()
		local bufname = vim.api.nvim_buf_get_name(0)
		if bufname ~= '' and vim.fn.isdirectory(bufname) == 1 then
			vim.fn.chdir(bufname)
			vim.api.nvim_buf_delete(0, { force = true })
			Snacks.picker.explorer({ cwd = bufname })
		end
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
