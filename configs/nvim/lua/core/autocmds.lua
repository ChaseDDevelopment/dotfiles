-- PackChanged hooks: run after vim.pack installs/updates plugins
vim.api.nvim_create_autocmd('PackChanged', {
	callback = function(ev)
		local name = ev.data.spec.name
		if name == 'nvim-treesitter' then
			if not ev.data.active then vim.cmd.packadd('nvim-treesitter') end
			vim.cmd('TSUpdate')
		elseif name == 'blink.cmp' then
			if vim.fn.executable('cargo') == 1 then
				local dir = vim.fn.stdpath('data') .. '/site/pack/core/opt/blink.cmp'
				vim.system({ 'cargo', 'build', '--release' }, { cwd = dir }):wait()
			else
				vim.notify('blink.cmp: cargo not found, fuzzy matching will use lua fallback', vim.log.levels.WARN)
			end
		end
	end,
})

-- Check for plugin updates in background (non-blocking git fetch)
vim.api.nvim_create_autocmd('UIEnter', {
	once = true,
	callback = function()
		vim.defer_fn(function()
			local pack_dir = vim.fn.stdpath('data') .. '/site/pack/core/opt'
			local plugins = vim.fn.readdir(pack_dir)
			local pending = #plugins
			local updatable = {}

			if pending == 0 then return end

			for _, name in ipairs(plugins) do
				local dir = pack_dir .. '/' .. name
				if vim.fn.isdirectory(dir .. '/.git') == 1 then
					vim.system({ 'git', 'fetch', '--quiet' }, { cwd = dir }, function(fetch_result)
						if fetch_result.code == 0 then
							vim.system(
								{ 'git', 'rev-list', '--count', 'HEAD..@{u}' },
								{ cwd = dir, text = true },
								function(count_result)
									local count = tonumber(vim.trim(count_result.stdout or '0')) or 0
									if count > 0 then
										table.insert(updatable, { name = name, commits = count })
									end
									pending = pending - 1
									if pending == 0 then
										vim.schedule(function()
											if #updatable > 0 then
												local lines = {}
												for _, p in ipairs(updatable) do
													table.insert(lines, ('  %s (%d new)'):format(p.name, p.commits))
												end
												vim.notify(
													('%d plugin(s) have updates:\n%s\nRun <leader>pu to update'):format(
														#updatable, table.concat(lines, '\n')),
													vim.log.levels.INFO
												)
											end
										end)
									end
								end
							)
						else
							pending = pending - 1
						end
					end)
				else
					pending = pending - 1
				end
			end
		end, 5000)
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
