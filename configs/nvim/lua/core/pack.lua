-- Plugin lifecycle for the vim.pack-based config (Neovim 0.12+).
--
-- Single source of truth for: which treesitter parsers we install, how
-- native-dependency plugins are (re)built, the launch-time update check,
-- and the headless bootstrap the dotfiles installer drives.
--
-- Two distinct build paths, deliberately:
--   * bootstrap()      — headless/installer, SYNCHRONOUS (:wait) so the
--                        installer only returns once everything is built.
--   * on_pack_changed  — interactive (<leader>pu approve), ASYNC with the
--                        plugins' own progress UI so the editor never freezes.
local M = {}

-- Treesitter PARSER names to keep installed. This is NOT the FileType list
-- (that lives in plugins/treesitter.lua and includes non-parser filetypes
-- like typescriptreact / yaml.ansible / terraform-vars). Keep them separate.
M.TS_LANGS = {
	"lua", "python", "typescript", "javascript", "rust", "yaml", "json",
	"markdown", "bash", "dockerfile", "tsx", "c_sharp", "toml", "xml", "html",
	"css", "hcl", "markdown_inline", "git_config", "gitcommit", "gitignore",
	"diff", "sql", "vim", "vimdoc", "luadoc", "regex",
}

-- Build failures collected during bootstrap(); surfaced (not swallowed) so
-- the installer can mark the run DEGRADED with the specific failing piece.
M.failures = {}

local function record_failure(what, err)
	local msg = what .. ": " .. tostring(err)
	table.insert(M.failures, msg)
	vim.notify(msg, vim.log.levels.WARN)
end

local function blink_dir()
	return vim.fn.stdpath("data") .. "/site/pack/core/opt/blink.cmp"
end

-- ── Synchronous builders (headless bootstrap) ──────────────────────────────

-- Install missing parsers and recompile any whose grammar revision changed.
-- The update() pass is what fixes stale parsers desynced from updated queries
-- (the "Invalid node type" class of error).
function M.build_treesitter()
	local ok, ts = pcall(require, "nvim-treesitter")
	if not ok then
		record_failure("nvim-treesitter load", ts)
		return
	end
	local ok_i, err_i = pcall(function() ts.install(M.TS_LANGS):wait(180000) end)
	if not ok_i then record_failure("treesitter install", err_i) end
	local ok_u, err_u = pcall(function() ts.update():wait(180000) end)
	if not ok_u then record_failure("treesitter update", err_u) end
end

-- Build blink.cmp's Rust fuzzy matcher. Prefer the plugin's own build()
-- (which uses blink.lib to fetch a prebuilt binary or compile), falling back
-- to a direct `cargo build --release`.
function M.build_blink()
	pcall(vim.cmd.packadd, "blink.lib")
	pcall(vim.cmd.packadd, "blink.cmp")
	local ok, cmp = pcall(require, "blink.cmp")
	if not ok then
		record_failure("blink.cmp load", cmp)
		return
	end
	if type(cmp.build) == "function" then
		local ok_b, err_b = pcall(function() cmp.build():wait(180000) end)
		if not ok_b then record_failure("blink.cmp build", err_b) end
		return
	end
	if vim.fn.executable("cargo") == 1 then
		local res = vim.system(
			{ "cargo", "build", "--release" },
			{ cwd = blink_dir() }
		):wait(180000)
		if res.code ~= 0 then
			record_failure("blink.cmp cargo build", res.stderr or ("exit " .. tostring(res.code)))
		end
	else
		record_failure("blink.cmp build", "cargo not found and blink.cmp has no build()")
	end
end

-- ── Interactive build (PackChanged after <leader>pu) ───────────────────────

-- Runs after vim.pack applies an install/update. Headless runs are handled by
-- bootstrap() explicitly, so this no-ops there. Builds async (no :wait) so the
-- UI is never blocked for the minutes a parser/Rust compile can take.
function M.on_pack_changed(ev)
	if #vim.api.nvim_list_uis() == 0 then
		return -- headless: bootstrap() owns building, synchronously
	end
	local kind = ev.data.kind
	if kind ~= "install" and kind ~= "update" then return end
	local name = ev.data.spec.name

	if name == "nvim-treesitter" then
		if not ev.data.active then pcall(vim.cmd.packadd, "nvim-treesitter") end
		pcall(vim.cmd, "TSUpdate") -- async, shows nvim-treesitter's progress
	elseif name == "blink.cmp" then
		if not ev.data.active then pcall(vim.cmd.packadd, "blink.cmp") end
		local ok, cmp = pcall(require, "blink.cmp")
		if ok and type(cmp.build) == "function" then
			pcall(cmp.build) -- async (no :wait → no UI freeze)
		elseif vim.fn.executable("cargo") == 1 then
			vim.system({ "cargo", "build", "--release" }, { cwd = blink_dir() })
		end
	end
end

-- ── Headless bootstrap (dotfiles installer) ────────────────────────────────

-- Bring every plugin to its latest tracked revision, then build all native
-- dependencies — synchronously, so the installer only returns once Neovim is
-- fully set up. Plugins are already cloned by init.lua's blocking
-- vim.pack.add(confirm=false); here we update + build.
function M.bootstrap()
	M.failures = {}

	-- Chase latest commits. vim.pack.update's exact sync/async timing isn't
	-- contractually guaranteed, so instead of counting PackChanged events we
	-- SETTLE-VERIFY: poll the (local, fast) offline view until nothing has a
	-- pending revision. Works whether update is sync or async; a plugin that
	-- can't reach its target just lets the wait time out and we build anyway.
	pcall(vim.pack.update, nil, { force = true })
	vim.wait(300000, function()
		local ok, infos = pcall(vim.pack.get, nil, { offline = true })
		if not ok then return true end
		for _, p in ipairs(infos) do
			if p.rev_to and p.rev_to ~= "" and p.rev ~= p.rev_to then
				return false
			end
		end
		return true
	end, 500)

	-- Always build (the deterministic safety floor): even when nothing
	-- updated, this recompiles stale parsers and builds a missing matcher.
	M.build_treesitter()
	M.build_blink()

	if #M.failures > 0 then
		io.stderr:write("nvim bootstrap completed with failures:\n")
		for _, f in ipairs(M.failures) do
			io.stderr:write("  - " .. f .. "\n")
		end
		-- Signal DEGRADED to the installer (headless only; cq exits non-zero).
		if #vim.api.nvim_list_uis() == 0 then
			pcall(vim.cmd, "cq")
		end
	end
	return M.failures
end

-- ── Launch-time update check (async, non-blocking) ─────────────────────────

-- Background git-fetch each managed plugin and report how many have upstream
-- commits. Intentionally NOT vim.pack.get(offline=false): that does a blocking
-- network fetch and would stall the UI on startup.
function M.check_updates()
	local pack_dir = vim.fn.stdpath("data") .. "/site/pack/core/opt"
	local plugins = vim.fn.readdir(pack_dir)
	local pending = #plugins
	local updatable = {}

	if pending == 0 then return end

	for _, name in ipairs(plugins) do
		local dir = pack_dir .. "/" .. name
		if vim.fn.isdirectory(dir .. "/.git") == 1 then
			vim.system({ "git", "fetch", "--quiet" }, { cwd = dir }, function(fetch_result)
				if fetch_result.code == 0 then
					vim.system(
						{ "git", "rev-list", "--count", "HEAD..@{u}" },
						{ cwd = dir, text = true },
						function(count_result)
							local count = tonumber(vim.trim(count_result.stdout or "0")) or 0
							if count > 0 then
								table.insert(updatable, { name = name, commits = count })
							end
							pending = pending - 1
							if pending == 0 then
								vim.schedule(function()
									if #updatable > 0 then
										local lines = {}
										for _, p in ipairs(updatable) do
											table.insert(lines, ("  %s (%d new)"):format(p.name, p.commits))
										end
										vim.notify(
											("%d plugin(s) have updates:\n%s\nRun <leader>pu to update"):format(
												#updatable, table.concat(lines, "\n")),
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
end

-- Register the PackChanged hook. Must run BEFORE vim.pack.add so install-time
-- events are caught.
function M.setup()
	vim.api.nvim_create_autocmd("PackChanged", { callback = M.on_pack_changed })
end

return M
