-- Single source of truth for nvim LSP server selection.
--
-- Each server is gated by the capability it needs to actually install and
-- run, so a minimal sysadmin host never attempts a doomed Mason install nor
-- spawns a missing binary (zero LSP error noise). Both mason.lua
-- (ensure_installed) and lsp.lua (vim.lsp.enable) consume this module.
--
-- Tiers (informational): the "base" sysadmin set installs everywhere; the
-- "dev" set only lights up where the matching SDK is present (the installer
-- only lays those down when "Install dev tools" is enabled).

local function has(bin)
	return vim.fn.executable(bin) == 1
end

local caps = {
	-- node-based LSPs: Mason installs them via npm. node alone is not
	-- enough — the Pi ships node without npm.
	npm = has("npm"),
	go = has("go"),
	dotnet = has("dotnet"),
}
-- A host counts as a developer box when a dev-gated SDK is present. The
-- installer only provisions these when "Install dev tools" is enabled, so
-- this is our proxy for "enable the developer-tier LSPs here". cargo/rustc
-- is deliberately excluded: it's a BASE tool (blink.cmp's Rust matcher),
-- present everywhere, so it can't signal a dev box.
caps.dev = caps.go or caps.dotnet or has("bun")

-- registry entries: { <lspconfig name>, cap = <gate>, uvbin = <exe> }
--   cap   = capability key required; nil means a zero-runtime prebuilt
--           binary that installs/runs anywhere.
--   uvbin = executable name for a server installed via `uv tool install`
--           (NOT Mason). Presence of the binary is the gate, and it is
--           excluded from Mason's ensure_installed.
--   dev   = true marks a uv server that should only enable on a dev box.
local registry = {
	-- BASE — zero-runtime prebuilt binaries (no language runtime)
	{ "lua_ls" },
	{ "taplo" },
	{ "marksman" },
	{ "terraformls" },
	-- BASE — node-based (Mason via npm)
	{ "yamlls", cap = "npm" },
	{ "jsonls", cap = "npm" },
	{ "bashls", cap = "npm" },
	{ "dockerls", cap = "npm" },
	{ "docker_compose_language_service", cap = "npm" },
	{ "ansiblels", cap = "npm" },
	{ "html", cap = "npm" },
	{ "cssls", cap = "npm" },
	-- BASE — Python LSPs installed via `uv tool install` (not Mason)
	{ "systemd_ls", uvbin = "systemd-language-server" },
	{ "nginx_language_server", uvbin = "nginx-language-server" },
	-- DEVELOPER — gated on the matching SDK / dev box
	{ "gopls", cap = "go" },
	-- rust_analyzer is dev-tier, but cargo is base (blink), so gate it on
	-- the dev-box heuristic rather than rustc-presence.
	{ "rust_analyzer", cap = "dev" },
	{ "omnisharp", cap = "dotnet" },
	{ "vtsls", cap = "dev" },
	{ "basedpyright", uvbin = "basedpyright-langserver", dev = true },
}

local function is_enabled(entry)
	if entry.uvbin then
		if entry.dev and not caps.dev then
			return false
		end
		return has(entry.uvbin)
	end
	if entry.cap == nil then
		return true
	end
	return caps[entry.cap] == true
end

local M = {}

-- LSP config names to hand to vim.lsp.enable().
function M.enabled_servers()
	local out = {}
	for _, e in ipairs(registry) do
		if is_enabled(e) then
			out[#out + 1] = e[1]
		end
	end
	return out
end

-- lspconfig names for mason-lspconfig ensure_installed: the enabled
-- servers that Mason actually manages (uv-installed servers excluded).
function M.mason_ensure()
	local out = {}
	for _, e in ipairs(registry) do
		if not e.uvbin and is_enabled(e) then
			out[#out + 1] = e[1]
		end
	end
	return out
end

return M
