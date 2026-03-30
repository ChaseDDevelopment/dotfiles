-- PackChanged hooks: run after vim.pack installs/updates plugins
vim.api.nvim_create_autocmd('PackChanged', {
	callback = function(ev)
		local name = ev.data.spec.name
		if name == 'nvim-treesitter' then
			if not ev.data.active then vim.cmd.packadd('nvim-treesitter') end
			vim.cmd('TSUpdate')
		elseif name == 'blink.cmp' then
			local dir = vim.fn.stdpath('data') .. '/site/pack/core/opt/blink.cmp'
			vim.system({ 'cargo', 'build', '--release' }, { cwd = dir }):wait()
		end
	end,
})

-- LSP keymaps: activate when a language server attaches to a buffer
vim.api.nvim_create_autocmd('LspAttach', {
	callback = function(ev)
		vim.keymap.set('n', '<leader>cr', vim.lsp.buf.rename, { buffer = ev.buf, desc = 'Rename symbol' })
		vim.keymap.set('n', '<leader>ca', vim.lsp.buf.code_action, { buffer = ev.buf, desc = 'Code action' })
		vim.keymap.set('n', 'K', vim.lsp.buf.hover, { buffer = ev.buf, desc = 'Hover docs' })
		vim.keymap.set('n', '<leader>cs', vim.lsp.buf.signature_help, { buffer = ev.buf, desc = 'Signature help' })
	end,
})
