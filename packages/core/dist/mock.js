import { extractMaker } from './modelMaker.js';
export { extractMaker } from './modelMaker.js';
import { EngineError } from './errors.js';
// Fixed base clock — the package contains no Date.now() and no randomness.
export const MOCK_NOW = '2026-01-01T12:00:00Z';
const clone = (v) => structuredClone(v);
const round2 = (x) => Math.round(x * 100) / 100;
// ---------------------------------------------------------------------------
// Fixtures (U01 CONTRACTS §4)
// ---------------------------------------------------------------------------
const GROUP_SLUGS = [
    'software_engineering',
    'reasoning',
    'knowledge',
    'research',
    'instruction_following',
    'agentic_tools',
    'evidence_capture',
    'ui_visual',
    'security',
    'data_ml',
    'finance',
];
// Verbatim from the mockup's GROUP_DEFS (demo.dc.html).
const GROUP_DEFS = [
    { slug: 'software_engineering', benchmarks: ['SWE-Bench Verified', 'SWE-Bench Pro', 'SWE-Bench Multilingual', 'SWE-Bench Multimodal', 'DeepSWE', 'Terminal-Bench', 'Terminal-Bench Hard', 'Aider Polyglot', 'SciCode', 'SWE-Atlas Codebase QnA', 'SWE-Atlas Test Writing', 'SWE-Atlas Refactoring', 'FrontierCode', 'FrontierSWE', 'NL2Repo', 'Program Bench', 'SWE Marathon', 'LiveCodeBench', 'LiveCodeBench Pro', 'MCP Atlas', 'Artificial Analysis Coding Index', 'Artificial Analysis Coding Agent Index', 'Toolathlon', 'AutomationBench'] },
    { slug: 'reasoning', benchmarks: ['GPQA Diamond', 'FrontierMath', 'ARC-AGI-2', 'AIME', 'HMMT'] },
    { slug: 'knowledge', benchmarks: ["Humanity's Last Exam", 'MMLU-Pro', 'MMMU Pro'] },
    { slug: 'research', benchmarks: ['BrowseComp', 'DeepSearchQA', 'WideSearch'] },
    { slug: 'instruction_following', benchmarks: ['IFBench', 'IFEval'] },
    { slug: 'agentic_tools', benchmarks: ['Terminal-Bench', 'Toolathlon', 'MCP Atlas', 'OSWorld-Verified'] },
    { slug: 'evidence_capture', benchmarks: ['OSWorld-Verified', 'Toolathlon', 'MCP Atlas', 'MMMU Pro'] },
    { slug: 'ui_visual', benchmarks: ['MMMU Pro', 'BabyVision', 'OmniDocBench', 'OSWorld-Verified'] },
    { slug: 'security', benchmarks: ['CyberGym', 'CTI-REALM'] },
    { slug: 'data_ml', benchmarks: ['DSBench-FullStack', 'DSBench-Hard', 'MLE-Bench', 'SpreadsheetBench'] },
    { slug: 'finance', benchmarks: ['Finance Agent', 'FinanceAgent', 'τ3 Banking', 'GDPval', 'GDPval-AA'] },
];
// HOME(b) = first §4.3 (builtin) group listing b.
const HOME = {};
for (const g of GROUP_DEFS) {
    for (const b of g.benchmarks) {
        if (!(b in HOME))
            HOME[b] = g.slug;
    }
}
// Verbatim from the mockup's ALL_BENCH, same order.
const ALL_BENCH = ['AA-Briefcase', 'AIME', 'ARC-AGI-1', 'ARC-AGI-2', 'ARC-AGI-3', "Agents' Last Exam", 'Aider Polyglot', 'Artificial Analysis Coding Agent Index', 'Artificial Analysis Coding Index', 'Artificial Analysis Intelligence Index', 'AutomationBench', 'BabyVision', 'BrowseComp', 'CTI-REALM', 'CharXiv Reasoning', 'ClawEval', 'CritPt', 'CyberGym', 'DSBench-FullStack', 'DSBench-Hard', 'DeepSWE', 'DeepSearchQA', 'FORTE', 'Finance Agent', 'FinanceAgent', 'Frontier-Bench', 'FrontierCode', 'FrontierMath', 'FrontierSWE', 'GDM-MRCR', 'GDPval', 'GDPval-AA', 'GPQA', 'GPQA Diamond', 'GeneBench', 'Graphwalks', 'HMMT', 'HealthBench Professional', "Humanity's Last Exam", 'IFBench', 'IFEval', 'IMOAnswerBench', 'JobBench', 'Kimi Claw 24/7 Bench', 'Kimi Code Bench', 'LiveCodeBench', 'LiveCodeBench Pro', 'Long Context Reasoning', 'MCP Atlas', 'MCP Mark Verified', 'MLE-Bench', 'MLS-Bench-Lite', 'MMLU', 'MMLU-Pro', 'MMMU Pro', 'MRCRv2', 'NL2Repo', 'OSWorld', 'OSWorld-Verified', 'OmniDocBench', 'OpenAI MRCR', 'PostTrainBench', 'Program Bench', 'SWE Bench Pro', 'SWE Marathon', 'SWE-Atlas Codebase QnA', 'SWE-Atlas Refactoring', 'SWE-Atlas Test Writing', 'SWE-Bench Multilingual', 'SWE-Bench Multimodal', 'SWE-Bench Pro', 'SWE-Bench Verified', 'SciCode', 'SpreadsheetBench', 'Terminal Bench 2.0', 'Terminal-Bench', 'Terminal-Bench 2.1', 'Terminal-Bench Hard', 'Tool-Decathlon', 'Toolathlon', 'Toolathlon-Verified', 'WideSearch', 'ZeroBench', 'τ3 Banking', 'τ²-Bench Telecom', 'τ³-Telecom'];
function mkModel(name, id, reasoning, providers, [intelligence, cost, speed], groupValues) {
    const groupScores = {};
    GROUP_SLUGS.forEach((slug, i) => {
        groupScores[slug] = groupValues[i];
    });
    return { id, name, reasoning, providers, core: { intelligence, cost, speed }, groupScores };
}
function seedModels() {
    return [
        mkModel('GPT-5.6 Luna', 'gpt-5.6-luna', 'max', ['codex', 'copilot'], [5.0, 3.0, 3.5], [4.9, 4.6, 4.6, 4.8, 4.4, 4.7, 4.4, 4.2, 4.0, 4.5, 4.3]),
        mkModel('Claude Opus 5', 'claude-opus-5', 'max', ['claude'], [4.9, 2.6, 3.2], [4.8, 4.8, 4.5, 4.6, 4.7, 4.9, 4.7, 4.6, 4.5, 4.4, 4.3]),
        mkModel('GPT-5.6 Sol', 'gpt-5.6-sol', 'high', ['copilot', 'codex'], [4.4, 4.0, 4.4], [4.3, 4.0, 4.2, 4.1, 4.2, 4.0, 4.0, 3.8, 3.9, 4.0, 3.9]),
        mkModel('Claude Sonnet 5.2', 'claude-sonnet-5.2', 'high', ['claude', 'copilot'], [4.2, 4.4, 4.6], [4.5, 4.1, 4.0, 3.9, 4.6, 4.4, 4.3, 4.4, 4.2, 4.0, 4.0]),
        mkModel('Gemini 3.5 Ultra', 'gemini-3.5-ultra', 'max', ['cursor'], [4.7, 3.4, 3.8], [4.4, 4.5, 4.8, 4.7, 4.0, 4.2, 4.2, 4.3, 4.1, 4.4, 4.1]),
        mkModel('Grok 5 Fast', 'grok-5-fast', 'medium', ['cursor', 'copilot'], [3.8, 4.7, 5.0], [4.0, 3.5, 3.6, 3.4, 3.9, 3.8, 3.6, 3.6, 3.4, 3.5, 3.4]),
        mkModel('Qwen 3.5 Max', 'qwen-3.5-max', 'medium', ['cursor'], [4.0, 4.9, 4.2], [4.1, 3.8, 4.0, 3.8, 3.7, 3.6, 3.6, 3.4, 3.5, 3.7, 3.5]),
        mkModel('Llama 5 405B', 'llama-5-405b', 'low', ['copilot'], [3.5, 5.0, 4.0], [3.5, 3.2, 3.6, 3.2, 3.4, 3.2, 3.2, 3.0, 3.2, 3.3, 3.1]),
    ];
}
function mkProfile(slug, name, coreShare, [intelligence, cost, speed], tier2, picks, lastUsed) {
    return {
        slug,
        name,
        builtin: true,
        core_share: coreShare,
        tier1_weights: { intelligence, cost, speed },
        tier2_weights: tier2,
        picks,
        last_used: lastUsed,
    };
}
const COMPLEXITY_SCALE = [
    'simple_action_execution',
    'simple_implementation',
    'balanced_implementation',
    'research',
    'planning',
];
function seedProfiles() {
    return [
        mkProfile('simple_action_execution', 'Simple Action', 75, [2, 5, 5], { instruction_following: 4, agentic_tools: 3 }, 312, '2026-01-01T11:48:00Z'),
        mkProfile('simple_implementation', 'Simple Implementation', 60, [4, 4, 3], { software_engineering: 4, instruction_following: 3, agentic_tools: 3 }, 1284, '2026-01-01T11:00:00Z'),
        mkProfile('balanced_implementation', 'Balanced Implementation', 70, [4, 3, 3], { software_engineering: 5, agentic_tools: 4, instruction_following: 3 }, 866, '2025-12-31T12:00:00Z'),
        mkProfile('research', 'Research', 60, [4, 4, 2], { research: 5, knowledge: 4, agentic_tools: 3 }, 174, '2025-12-29T12:00:00Z'),
        mkProfile('planning', 'Planning', 60, [5, 2, 2], { reasoning: 5, research: 4, knowledge: 3 }, 121, ''),
        mkProfile('review', 'Review', 65, [4, 3, 3], { instruction_following: 5, security: 3 }, 58, ''),
        mkProfile('ui_ux', 'UI / UX', 60, [4, 3, 3], { ui_visual: 5, software_engineering: 4 }, 43, ''),
        mkProfile('research_fast', 'Research (fast)', 60, [3, 4, 5], { research: 5, knowledge: 3 }, 19, ''),
    ];
}
const PROVIDER_IDS = ['claude', 'codex', 'copilot', 'cursor'];
function mkHarness(slug, name, command, installed, providersOn, enabled = installed) {
    const providers = {};
    for (const id of PROVIDER_IDS)
        providers[id] = providersOn.includes(id);
    return { slug, name, command, builtin: true, installed, enabled, providers };
}
function seedHarnesses() {
    return [
        mkHarness('claude', 'Claude Code', 'claude --model {model_id}', true, ['claude', 'codex', 'copilot']),
        mkHarness('codex', 'Codex CLI', 'codex -m {model_id}', true, ['codex', 'copilot']),
        mkHarness('copilot', 'Copilot CLI', 'copilot --model {model_id}', true, ['copilot', 'cursor']),
        mkHarness('cursor', 'Cursor Agent', 'cursor-agent --model {model_id}', false, ['cursor']),
        mkHarness('aider', 'Aider', 'aider --model {model_id}', true, ['claude', 'codex', 'copilot', 'cursor']),
        mkHarness('goose', 'Goose', 'goose session --model {model_id}', false, ['claude', 'codex', 'copilot']),
        mkHarness('windsurf', 'Windsurf', 'windsurf', false, ['claude', 'codex', 'copilot', 'cursor']),
        mkHarness('amp', 'Amp', 'amp', false, []),
        mkHarness('antigravity', 'Antigravity', 'agy --model {model_id}', false, []),
        mkHarness('cline', 'Cline', 'cline --model {model_id}', true, ['claude', 'codex']),
        mkHarness('continue', 'Continue', 'cn', false, []),
        mkHarness('crush', 'Crush', 'crush', false, []),
        mkHarness('droid', 'Factory Droid', 'droid --model {model_id}', false, []),
        mkHarness('gemini', 'Gemini CLI', 'gemini --model {model_id}', false, []),
        mkHarness('kilo', 'Kilo Code', 'kilo --model {model_id}', false, []),
        mkHarness('kiro', 'Kiro CLI', 'kiro-cli chat', false, []),
        mkHarness('opencode', 'OpenCode', 'opencode --model {model_id}', true, ['claude', 'codex', 'copilot']),
        mkHarness('qwen', 'Qwen Code', 'qwen --model {model_id}', false, []),
    ];
}
const MOCK_COSTS = {
    'claude/claude-opus-5': { input: 15, output: 75 },
    'codex/gpt-5.6-luna': { input: 2.5, output: 10 },
    'codex/gpt-5.6-sol': { input: 1.25, output: 10 },
};
function seedProviders() {
    return [
        { id: 'claude', on: true, priority: 1, auth: 'oauth', limits: 'session 42% · weekly 18%', session: 42, weekly: 18, monthly: 54, credits: 'max 20× plan', resets: 'session in 2h 40m' },
        { id: 'codex', on: true, priority: 2, auth: 'oauth', limits: 'session 12% · weekly 31% · 340 credits', session: 12, weekly: 31, monthly: 44, credits: '340 credits left', resets: 'weekly on Mon' },
        { id: 'copilot', on: true, priority: 3, auth: 'device flow', limits: 'monthly 1200 of 4800', session: 8, weekly: 25, monthly: 25, credits: '1200 of 4800 premium', resets: 'monthly on the 1st' },
        { id: 'cursor', on: false, priority: 4, auth: 'oauth', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no plan detected', resets: '—' },
        { id: 'antigravity', on: false, priority: 5, auth: 'oauth', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no plan detected', resets: '—' },
        { id: 'commandcode', on: false, priority: 6, auth: 'via codexbar', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no plan detected', resets: '—' },
        { id: 'google', on: false, priority: 7, auth: 'custom', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no usage data', resets: '—' },
        { id: 'mistral', on: false, priority: 8, auth: 'custom', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no usage data', resets: '—' },
        { id: 'xai', on: false, priority: 9, auth: 'custom', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no usage data', resets: '—' },
    ];
}
function seedSettings() {
    return {
        layout: 'carousel',
        default_tab: 'profiles',
        weight_control: 'slider',
        holds: 5,
        shortcut: 'alt+space',
        show_menu_bar_icon: true,
        launch_at_login: false,
        copy_command_instead: false,
        close_popover_after_launch: true,
        auto_update: true,
        auto_update_frequency: 'daily',
        mcp_server: false,
        claude_md_hint: false,
        shell_alias: false,
        use_keychain: true,
        catalog_repo: 'WD-Mitchell/which-model',
        use_local_aa: false,
        only_enabled_providers: false,
        benchmark_check_frequency: '6h',
        aa_api_key: '',
        aa_api_key_set: false,
        config_path: '~/Library/Application Support/which-model/config.toml',
        version: 'which-model dev (commit unknown, built unknown)',
    };
}
function seedData() {
    return {
        profiles: seedProfiles(),
        models: seedModels(),
        groups: GROUP_DEFS.map((g) => ({ slug: g.slug, builtin: true, benchmarks: [...g.benchmarks] })),
        benchmarks: [...ALL_BENCH],
        harnesses: seedHarnesses(),
        providers: seedProviders(),
        favourites: [],
        routesDisabled: [],
        settings: seedSettings(),
    };
}
// ---------------------------------------------------------------------------
// Route keys (D00 CONTRACTS §1)
// ---------------------------------------------------------------------------
const ROUTE_KEY_RE = /^([a-z0-9][a-z0-9_-]*)\/([A-Za-z0-9._-]+)@(minimal|low|medium|high|xhigh|max|default)$/;
const SLUG_RE = /^[a-z0-9_]+$/;
function parseRouteKey(key) {
    const m = ROUTE_KEY_RE.exec(key);
    if (!m) {
        throw new EngineError('validation_failed', `invalid route key "${key}"`);
    }
    return { provider: m[1], modelId: m[2], reasoning: m[3] };
}
function formatRouteKey(provider, modelId, reasoning) {
    return `${provider}/${modelId}@${reasoning}`;
}
const EFFORT_ORDER = ['minimal', 'low', 'medium', 'high', 'xhigh', 'max'];
function collapseReasoning(level) {
    return level === 'default' ? 'high' : level;
}
function addReasoningLevel(levels, reasoning) {
    const collapsed = collapseReasoning(reasoning);
    if (levels.some((existing) => collapseReasoning(existing) === collapsed))
        return;
    levels.push(reasoning);
}
function reasoningLess(left, right) {
    const leftCanonical = collapseReasoning(left);
    const rightCanonical = collapseReasoning(right);
    const leftRank = EFFORT_ORDER.indexOf(leftCanonical);
    const rightRank = EFFORT_ORDER.indexOf(rightCanonical);
    const leftKnown = leftRank >= 0;
    const rightKnown = rightRank >= 0;
    if (leftKnown && rightKnown && leftRank !== rightRank)
        return leftRank < rightRank;
    if (leftKnown !== rightKnown)
        return leftKnown;
    if (leftCanonical !== rightCanonical)
        return leftCanonical < rightCanonical;
    return left < right;
}
function topReasoning(levels) {
    let top = '';
    let topRank = -1;
    for (const level of levels) {
        const canonical = collapseReasoning(level);
        const rank = EFFORT_ORDER.indexOf(canonical);
        if (rank < 0) {
            if (top === '')
                top = canonical;
            continue;
        }
        if (rank > topRank) {
            topRank = rank;
            top = canonical;
        }
    }
    return top;
}
/** Currently offered models + effort levels — independent of scores. */
const PROVIDER_CATALOGUE = {
    claude: [
        { id: 'claude-haiku-4', name: 'Claude Haiku 4', levels: ['low', 'medium', 'high'] },
        { id: 'claude-opus-5', name: 'Claude Opus 5', levels: ['low', 'high', 'max'] },
        { id: 'claude-sonnet-5.2', name: 'Claude Sonnet 5.2', levels: ['low', 'medium', 'high'] },
    ],
    codex: [
        { id: 'gpt-5.6-luna', name: 'GPT-5.6 Luna', levels: ['low', 'medium', 'high', 'max'] },
        { id: 'gpt-5.6-sol', name: 'GPT-5.6 Sol', levels: ['low', 'medium', 'high'] },
    ],
    copilot: [
        { id: 'claude-opus-5', name: 'Claude Opus 5', levels: ['low', 'medium', 'high', 'max'] },
        { id: 'claude-sonnet-5.2', name: 'Claude Sonnet 5.2', levels: ['low', 'medium', 'high'] },
        { id: 'gpt-5.6-luna', name: 'GPT-5.6 Luna', levels: ['low', 'medium', 'high', 'max'] },
        { id: 'gpt-5.6-sol', name: 'GPT-5.6 Sol', levels: ['low', 'medium', 'high'] },
        { id: 'grok-5-fast', name: 'Grok 5 Fast', levels: ['low', 'medium', 'high'] },
        { id: 'llama-5-405b', name: 'Llama 5 405B', levels: ['low', 'medium'] },
    ],
    cursor: [
        { id: 'gemini-3.5-ultra', name: 'Gemini 3.5 Ultra', levels: ['low', 'medium', 'high', 'max'] },
        { id: 'grok-5-fast', name: 'Grok 5 Fast', levels: ['low', 'medium', 'high'] },
        { id: 'qwen-3.5-max', name: 'Qwen 3.5 Max', levels: ['low', 'medium', 'high'] },
    ],
};
// ---------------------------------------------------------------------------
// Scoring (U01 CONTRACTS §7)
// ---------------------------------------------------------------------------
function benchScore(m, bench) {
    const home = HOME[bench];
    return (home !== undefined ? m.groupScores[home] : undefined) ?? 3.7;
}
function groupScore(m, benchmarks) {
    if (benchmarks.length === 0)
        return 3.5;
    let sum = 0;
    for (const b of benchmarks)
        sum += benchScore(m, b);
    return sum / benchmarks.length;
}
function scoreModel(data, m, p) {
    let coreNum = 0;
    let coreDen = 0;
    for (const [key, w] of Object.entries(p.tier1_weights)) {
        if (w > 0) {
            coreNum += w * (m.core[key] ?? 0);
            coreDen += w * 5;
        }
    }
    const coreRatio = coreDen > 0 ? coreNum / coreDen : 0.7;
    let taskNum = 0;
    let taskDen = 0;
    for (const g of data.groups) {
        const w = p.tier2_weights[g.slug] ?? 0;
        if (w > 0) {
            taskNum += w * groupScore(m, g.benchmarks);
            taskDen += w * 5;
        }
    }
    const taskRatio = taskDen > 0 ? taskNum / taskDen : 0.7;
    const cs = p.core_share / 100;
    return 100 * (cs * coreRatio + (1 - cs) * taskRatio);
}
// ---------------------------------------------------------------------------
// Mock host
// ---------------------------------------------------------------------------
export function createMockEngineHost(overrides) {
    const data = { ...seedData(), ...(overrides ? clone(overrides) : {}) };
    // Per-host accounts so tests cannot leak signed-in state into later cases.
    let nextSignInFlow = 0;
    const activeSignInFlows = new Map();
    const confirmWaiters = new Map();
    const mockAccounts = {
        claude: [{ name: 'Work', kind: 'oauth', ref: '~/.claude/.credentials.json' }],
        codex: [{ name: 'ChatGPT', kind: 'oauth', ref: '' }],
        copilot: [{ name: 'GitHub', kind: 'oauth', ref: '' }],
    };
    let usageMode = 'auto';
    let usageBackend = 'native';
    const listeners = new Map();
    function emit(event, payload) {
        const set = listeners.get(event);
        if (!set)
            return;
        // Iterate a snapshot so unsubscribing during dispatch is safe.
        for (const cb of [...set]) {
            if (set.has(cb))
                cb(clone(payload));
        }
    }
    const notFound = (kind, id) => new EngineError('not_found', `${kind} "${id}" does not exist`);
    function requireProfile(slug) {
        const p = data.profiles.find((x) => x.slug === slug);
        if (!p)
            throw notFound('profile', slug);
        return p;
    }
    function requireGroup(slug) {
        const g = data.groups.find((x) => x.slug === slug);
        if (!g)
            throw notFound('group', slug);
        return g;
    }
    function requireProvider(id) {
        const p = data.providers.find((x) => x.id === id);
        if (!p)
            throw notFound('provider', id);
        return p;
    }
    function requireHarness(slug) {
        const h = data.harnesses.find((x) => x.slug === slug);
        if (!h)
            throw notFound('harness', slug);
        return h;
    }
    function freeSlug(base, taken) {
        let candidate = `${base}_copy`;
        for (let n = 2; taken(candidate); n++)
            candidate = `${base}_copy_${n}`;
        return candidate;
    }
    function routeDisabled(provider, modelId, reasoning) {
        return data.routesDisabled.includes(formatRouteKey(provider, modelId, reasoning));
    }
    function providerModels(id) {
        const models = new Map();
        for (const row of data.models.filter((m) => m.providers.includes(id))) {
            const cur = models.get(row.id);
            if (cur === undefined)
                models.set(row.id, { name: row.name, levels: [row.reasoning] });
            else
                addReasoningLevel(cur.levels, row.reasoning);
        }
        for (const entry of PROVIDER_CATALOGUE[id] ?? []) {
            const cur = models.get(entry.id);
            if (cur === undefined) {
                const levels = [...entry.levels];
                if (levels.length === 0)
                    addReasoningLevel(levels, 'default');
                models.set(entry.id, { name: entry.name, levels });
                continue;
            }
            if (entry.levels.length > 0) {
                for (const level of entry.levels)
                    addReasoningLevel(cur.levels, level);
            }
            else if (cur.levels.length === 0) {
                addReasoningLevel(cur.levels, 'default');
            }
        }
        return [...models.entries()]
            .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
            .map(([modelId, model]) => {
            const levels = [...model.levels].sort((a, b) => (reasoningLess(a, b) ? -1 : 1));
            const top = topReasoning(levels);
            let defaultSet = false;
            return {
                model_id: modelId,
                model_name: model.name,
                levels: levels.map((reasoning) => {
                    const canonical = collapseReasoning(reasoning);
                    const isDefault = defaultSet === false && canonical === top;
                    if (isDefault)
                        defaultSet = true;
                    return {
                        reasoning,
                        enabled: routeDisabled(id, modelId, reasoning) === false,
                        default: isDefault,
                    };
                }),
            };
        });
    }
    function providersByPriority() {
        return [...data.providers].sort((a, b) => a.priority - b.priority);
    }
    // Candidate route for a model: first enabled provider in priority order that
    // the model lists and whose route is not disabled; null when none.
    function routeFor(m) {
        for (const p of providersByPriority()) {
            if (p.on && m.providers.includes(p.id) && !routeDisabled(p.id, m.id, m.reasoning)) {
                return p.id;
            }
        }
        return null;
    }
    function computeRank(profile, holds) {
        const scored = [];
        for (const m of data.models) {
            const provider = routeFor(m);
            if (provider === null)
                continue;
            scored.push({ m, provider, score: round2(scoreModel(data, m, profile)) });
        }
        scored.sort((a, b) => b.score - a.score || (a.m.id < b.m.id ? -1 : a.m.id > b.m.id ? 1 : 0));
        const total = scored.length;
        const candidates = scored.slice(0, holds).map((c, i) => ({
            rank: i + 1,
            model_id: c.m.id,
            model_name: c.m.name,
            provider: c.provider,
            reasoning: c.m.reasoning,
            score: c.score,
            route_key: formatRouteKey(c.provider, c.m.id, c.m.reasoning),
            intelligence: c.m.core.intelligence,
            cost: c.m.core.cost,
            speed: c.m.core.speed,
        }));
        return { candidates, total };
    }
    function recordPickInternal(profileSlug, routeKey) {
        parseRouteKey(routeKey);
        const profile = data.profiles.find((p) => p.slug === profileSlug);
        if (profile) {
            profile.picks += 1;
            profile.last_used = MOCK_NOW;
        }
        emit('pick:recorded', { profile_slug: profileSlug, route_key: routeKey });
    }
    function groupDetailFor(g) {
        return {
            slug: g.slug,
            builtin: g.builtin,
            benchmarks: data.benchmarks.map((name) => ({
                name,
                on: g.benchmarks.includes(name),
                covered: 8,
                coverage_total: 8,
            })),
        };
    }
    const host = {
        data,
        profiles: {
            async list() {
                return clone(data.profiles);
            },
            async get(slug) {
                return clone(requireProfile(slug));
            },
            async create(p) {
                if (!SLUG_RE.test(p.slug))
                    throw new EngineError('validation_failed', `invalid profile slug "${p.slug}"`);
                if (data.profiles.some((x) => x.slug === p.slug))
                    throw new EngineError('conflict', `profile "${p.slug}" already exists`);
                data.profiles.push({ ...clone(p), builtin: false, picks: 0, last_used: '' });
                emit('config:changed', { section: 'profiles' });
            },
            async save(p) {
                if (!SLUG_RE.test(p.slug)) {
                    throw new EngineError('validation_failed', `invalid profile slug "${p.slug}"`);
                }
                const existing = data.profiles.find((x) => x.slug === p.slug);
                if (existing?.builtin) {
                    throw new EngineError('builtin_readonly', `profile "${p.slug}" is built-in and read-only`);
                }
                const saved = { ...clone(p), builtin: false };
                if (existing) {
                    data.profiles[data.profiles.indexOf(existing)] = saved;
                }
                else {
                    data.profiles.push(saved);
                }
                emit('config:changed', { section: 'profiles' });
            },
            async duplicate(slug) {
                const src = requireProfile(slug);
                const copy = {
                    ...clone(src),
                    slug: freeSlug(src.slug, (s) => data.profiles.some((p) => p.slug === s)),
                    name: `${src.name} copy`,
                    builtin: false,
                    picks: 0,
                    last_used: '',
                };
                data.profiles.push(copy);
                emit('config:changed', { section: 'profiles' });
                return clone(copy);
            },
            async delete(slug) {
                const p = requireProfile(slug);
                if (p.builtin) {
                    throw new EngineError('builtin_readonly', `profile "${slug}" is built-in and read-only`);
                }
                data.profiles.splice(data.profiles.indexOf(p), 1);
                emit('config:changed', { section: 'profiles' });
            },
            async complexityScale() {
                return [...COMPLEXITY_SCALE];
            },
        },
        pick: {
            async rank(req) {
                const profile = req.overrides ?? requireProfile(req.profile_slug);
                const holds = req.holds !== 0 ? req.holds : data.settings.holds;
                if (holds !== 1 && holds !== 3 && holds !== 5) {
                    throw new EngineError('validation_failed', `holds ${holds} must be 1, 3 or 5`);
                }
                return clone(computeRank(profile, holds));
            },
            async recordPick(profileSlug, routeKey) {
                recordPickInternal(profileSlug, routeKey);
            },
            async catalogLine() {
                return {
                    models: data.models.length,
                    providers_on: data.providers.filter((p) => p.on).length,
                    harnesses: data.harnesses.length,
                };
            },
        },
        catalog: {
            async benchmarks() {
                return clone(data.benchmarks);
            },
            async benchmarkDetail(name) {
                if (!data.benchmarks.includes(name))
                    throw notFound('benchmark', name);
                const rows = data.models.map((m) => {
                    const value = round2(benchScore(m, name) * 20);
                    return { model: m.name, reasoning: m.reasoning, value };
                });
                const maxValue = Math.max(...rows.map((r) => r.value));
                const withNorm = rows.map((r) => ({
                    ...r,
                    norm: Math.round((r.value / maxValue) * 100),
                }));
                withNorm.sort((a, b) => b.norm - a.norm || (a.model < b.model ? -1 : a.model > b.model ? 1 : 0));
                return {
                    name,
                    note: '',
                    groups: data.groups.filter((g) => g.benchmarks.includes(name)).map((g) => g.slug),
                    rows: withNorm,
                };
            },
            async modelDetail(model, reasoning) {
                const match = data.models.find((x) => x.name === model && x.reasoning === reasoning);
                if (match === undefined) {
                    return { model, reasoning, rows: [] };
                }
                const rows = data.benchmarks.map((name) => {
                    const values = data.models.map((row) => round2(benchScore(row, name) * 20));
                    const maxValue = Math.max(...values, 0);
                    const value = round2(benchScore(match, name) * 20);
                    return {
                        name,
                        value,
                        norm: maxValue === 0 ? 0 : Math.round((value / maxValue) * 100),
                        groups: data.groups.filter((g) => g.benchmarks.includes(name)).map((g) => g.slug),
                    };
                });
                rows.sort((a, b) => b.norm - a.norm || a.name.localeCompare(b.name));
                return { model, reasoning, rows };
            },
            async models() {
                const byName = new Map();
                const onProviders = new Set(data.providers.filter((p) => p.on).map((p) => p.id));
                for (const m of data.models) {
                    const rank = EFFORT_ORDER.indexOf(collapseReasoning(m.reasoning));
                    let acc = byName.get(m.name);
                    const mProviders = m.providers.filter((p) => onProviders.has(p));
                    if (acc === undefined) {
                        byName.set(m.name, {
                            name: m.name,
                            id: m.id,
                            reasoning: [m.reasoning],
                            intel: m.core.intelligence,
                            cost: m.core.cost,
                            speed: m.core.speed,
                            providers: new Set(mProviders),
                            topRank: rank,
                        });
                        continue;
                    }
                    if (!acc.reasoning.includes(m.reasoning))
                        acc.reasoning.push(m.reasoning);
                    for (const provider of mProviders)
                        acc.providers.add(provider);
                    if (rank >= acc.topRank) {
                        acc.topRank = rank;
                        acc.id = m.id;
                        acc.intel = m.core.intelligence;
                        acc.cost = m.core.cost;
                        acc.speed = m.core.speed;
                    }
                }
                for (const p of data.providers) {
                    if (!p.on)
                        continue;
                    for (const pm of providerModels(p.id)) {
                        let acc = byName.get(pm.model_name);
                        if (!acc) {
                            acc = { name: pm.model_name, id: pm.model_id, reasoning: [], intel: null, cost: null, speed: null, providers: new Set(), topRank: -1 };
                            byName.set(pm.model_name, acc);
                        }
                        for (const lvl of pm.levels)
                            acc.reasoning.push(lvl.reasoning);
                        acc.providers.add(p.id);
                    }
                }
                const list = [...byName.values()]
                    .sort((a, b) => a.name.localeCompare(b.name))
                    .map((a) => ({
                    model_name: a.name,
                    model_id: a.id,
                    reasoning: [...new Set(a.reasoning)].sort((x, y) => (reasoningLess(x, y) ? -1 : 1)),
                    intelligence: a.intel,
                    cost: a.cost,
                    speed: a.speed,
                    provider_count: a.providers.size,
                    maker: extractMaker(a.name),
                    providers: [...a.providers].sort(),
                }));
                if (data.settings.only_enabled_providers) {
                    return list.filter((m) => m.provider_count > 0);
                }
                return list;
            },
            async model(name) {
                const eq = (a, b) => a.localeCompare(b, undefined, { sensitivity: 'accent' }) === 0;
                const rows = data.models.filter((m) => eq(m.name, name) || eq(m.id, name));
                const enabledProviders = data.providers.filter((p) => p.on);
                const groups = new Map();
                const allReasoning = new Set();
                if (rows.length === 0) {
                    const matchingProviderModels = [];
                    for (const p of data.providers) {
                        for (const pm of providerModels(p.id)) {
                            if (eq(pm.model_name, name) || eq(pm.model_id, name)) {
                                const scoredMatch = data.models.filter((m) => eq(m.name, pm.model_name));
                                if (scoredMatch.length > 0) {
                                    rows.push(...scoredMatch);
                                    break;
                                }
                                matchingProviderModels.push({ provider: p.id, model: pm });
                                for (const level of pm.levels) {
                                    allReasoning.add(collapseReasoning(level.reasoning));
                                }
                            }
                        }
                        if (rows.length > 0)
                            break;
                    }
                    if (rows.length === 0 && matchingProviderModels.length === 0) {
                        throw notFound('model', name);
                    }
                    if (rows.length === 0) {
                        for (const p of enabledProviders) {
                            for (const pm of providerModels(p.id)) {
                                if (eq(pm.model_name, name) || eq(pm.model_id, name)) {
                                    const join = `${p.id}|${pm.model_id}`;
                                    let acc = groups.get(join);
                                    if (acc === undefined) {
                                        acc = { provider: p.id, model_id: pm.model_id, reasoning: new Set(), keys: new Set() };
                                        groups.set(join, acc);
                                    }
                                    for (const level of pm.levels) {
                                        const collapsed = collapseReasoning(level.reasoning);
                                        acc.reasoning.add(collapsed);
                                        allReasoning.add(collapsed);
                                        acc.keys.add(formatRouteKey(p.id, pm.model_id, collapsed));
                                    }
                                }
                            }
                        }
                        const providers = [...groups.values()]
                            .sort((a, b) => a.provider.localeCompare(b.provider) || a.model_id.localeCompare(b.model_id))
                            .map((a) => {
                            const cost = MOCK_COSTS[`${a.provider}/${a.model_id}`];
                            return {
                                provider: a.provider,
                                model_id: a.model_id,
                                reasoning: [...a.reasoning].sort((x, y) => (reasoningLess(x, y) ? -1 : 1)),
                                route_keys: [...a.keys].sort(),
                                input_cost_usd_per_m: cost === undefined ? null : cost.input,
                                output_cost_usd_per_m: cost === undefined ? null : cost.output,
                            };
                        });
                        const first = matchingProviderModels[0].model;
                        return {
                            model_name: first.model_name,
                            model_id: first.model_id,
                            reasoning: [...allReasoning].sort((x, y) => (reasoningLess(x, y) ? -1 : 1)),
                            intelligence: null,
                            cost: null,
                            speed: null,
                            provider_count: new Set(providers.map((pr) => pr.provider)).size,
                            in_catalog: false,
                            providers,
                        };
                    }
                }
                for (const m of rows) {
                    allReasoning.add(m.reasoning);
                }
                const top = [...rows].sort((a, b) => EFFORT_ORDER.indexOf(collapseReasoning(b.reasoning)) -
                    EFFORT_ORDER.indexOf(collapseReasoning(a.reasoning)))[0];
                for (const p of enabledProviders) {
                    for (const pm of providerModels(p.id)) {
                        if (eq(pm.model_name, top.name) || eq(pm.model_id, top.id)) {
                            const join = `${p.id}|${pm.model_id}`;
                            let acc = groups.get(join);
                            if (acc === undefined) {
                                acc = { provider: p.id, model_id: pm.model_id, reasoning: new Set(), keys: new Set() };
                                groups.set(join, acc);
                            }
                            for (const level of pm.levels) {
                                const collapsed = collapseReasoning(level.reasoning);
                                acc.reasoning.add(collapsed);
                                allReasoning.add(collapsed);
                                acc.keys.add(formatRouteKey(p.id, pm.model_id, collapsed));
                            }
                        }
                    }
                }
                for (const m of rows) {
                    for (const p of enabledProviders) {
                        if (m.providers.includes(p.id)) {
                            const join = `${p.id}|${m.id}`;
                            let acc = groups.get(join);
                            if (acc === undefined) {
                                acc = { provider: p.id, model_id: m.id, reasoning: new Set(), keys: new Set() };
                                groups.set(join, acc);
                            }
                            const collapsed = collapseReasoning(m.reasoning);
                            acc.reasoning.add(collapsed);
                            allReasoning.add(collapsed);
                            acc.keys.add(formatRouteKey(p.id, m.id, collapsed));
                        }
                    }
                }
                const providers = [...groups.values()]
                    .sort((a, b) => a.provider.localeCompare(b.provider) || a.model_id.localeCompare(b.model_id))
                    .map((a) => {
                    const cost = MOCK_COSTS[`${a.provider}/${a.model_id}`];
                    return {
                        provider: a.provider,
                        model_id: a.model_id,
                        reasoning: [...a.reasoning].sort((x, y) => (reasoningLess(x, y) ? -1 : 1)),
                        route_keys: [...a.keys].sort(),
                        input_cost_usd_per_m: cost === undefined ? null : cost.input,
                        output_cost_usd_per_m: cost === undefined ? null : cost.output,
                    };
                });
                return {
                    model_name: top.name,
                    model_id: top.id,
                    reasoning: [...allReasoning].sort((x, y) => (reasoningLess(x, y) ? -1 : 1)),
                    intelligence: top.core.intelligence,
                    cost: top.core.cost,
                    speed: top.core.speed,
                    provider_count: new Set(rows.flatMap((m) => m.providers)).size,
                    in_catalog: true,
                    providers,
                };
            },
            async groups() {
                return data.groups.map((g) => ({
                    slug: g.slug,
                    builtin: g.builtin,
                    benchmark_count: g.benchmarks.length,
                    in_profiles: data.profiles.filter((p) => (p.tier2_weights[g.slug] ?? 0) > 0).length,
                }));
            },
            async groupDetail(slug) {
                return groupDetailFor(requireGroup(slug));
            },
            async saveGroup(slug, benchmarks, renameTo) {
                const g = requireGroup(slug);
                if (g.builtin) {
                    throw new EngineError('builtin_readonly', `group "${slug}" is built-in and read-only`);
                }
                for (const b of benchmarks) {
                    if (!data.benchmarks.includes(b)) {
                        throw new EngineError('validation_failed', `unknown benchmark "${b}"`);
                    }
                }
                if (renameTo !== undefined && renameTo !== slug) {
                    if (!SLUG_RE.test(renameTo)) {
                        throw new EngineError('validation_failed', `invalid group slug "${renameTo}"`);
                    }
                    if (data.groups.some((x) => x.slug === renameTo)) {
                        throw new EngineError('conflict', `group "${renameTo}" already exists`);
                    }
                    g.slug = renameTo;
                }
                g.benchmarks = [...benchmarks];
                emit('catalog:changed', {});
            },
            async duplicateGroup(slug) {
                const src = requireGroup(slug);
                const copy = {
                    slug: freeSlug(src.slug, (s) => data.groups.some((g) => g.slug === s)),
                    builtin: false,
                    benchmarks: [...src.benchmarks],
                };
                data.groups.push(copy);
                emit('catalog:changed', {});
                return groupDetailFor(copy);
            },
            async deleteGroup(slug) {
                const g = requireGroup(slug);
                if (g.builtin) {
                    throw new EngineError('builtin_readonly', `group "${slug}" is built-in and read-only`);
                }
                data.groups.splice(data.groups.indexOf(g), 1);
                emit('catalog:changed', {});
            },
        },
        providers: {
            async add(id) {
                if (!data.providers.some((p) => p.id === id)) {
                    data.providers.push({
                        id,
                        on: false,
                        priority: data.providers.length + 1,
                        auth: 'custom',
                        limits: 'not enabled',
                        session: null,
                        weekly: null,
                        monthly: null,
                        credits: 'no usage data',
                        resets: '—',
                    });
                    emit('config:changed', { section: 'providers' });
                }
            },
            async addable() {
                return [];
            },
            async delete(_id) { },
            async duplicate(id) {
                return `${id}_2`;
            },
            async setAccounts(id, accounts) {
                mockAccounts[id] = accounts.map((a) => ({ ...a }));
                emit('config:changed', { section: 'providers' });
            },
            async list() {
                return providersByPriority().map((p) => {
                    const models = data.models.filter((m) => m.providers.includes(p.id));
                    return {
                        id: p.id,
                        enabled: p.on,
                        priority: p.priority,
                        models: new Set(models.map((model) => model.id)).size,
                        auth: p.auth,
                        limits_line: p.limits,
                        routes_on: models.filter((m) => !routeDisabled(p.id, m.id, m.reasoning)).length,
                        routes_total: models.length,
                        session: p.session,
                        weekly: p.weekly,
                        monthly: p.monthly,
                        credits: p.credits,
                        resets: p.resets,
                        accounts: (mockAccounts[p.id] ?? []).length,
                        // Mirrors the real native and CodexBar provider registries.
                        builtin: ['antigravity', 'claude', 'codex', 'commandcode', 'copilot', 'cursor'].includes(p.id),
                    };
                });
            },
            async setEnabled(id, on) {
                requireProvider(id).on = on;
                emit('config:changed', { section: 'providers' });
            },
            async reorder(orderedIds) {
                for (const id of orderedIds)
                    requireProvider(id);
                if (new Set(orderedIds).size !== data.providers.length) {
                    throw new EngineError('validation_failed', 'reorder must list every provider exactly once');
                }
                orderedIds.forEach((id, i) => {
                    requireProvider(id).priority = i + 1;
                });
                emit('config:changed', { section: 'providers' });
            },
            async detail(id) {
                const p = requireProvider(id);
                const detail = {
                    id: p.id,
                    accounts: mockAccounts[p.id] ?? [],
                    oauth_supported: ['antigravity', 'claude', 'codex', 'copilot', 'cursor'].includes(p.id),
                    builtin: ['antigravity', 'claude', 'codex', 'commandcode', 'copilot', 'cursor'].includes(p.id),
                    models: providerModels(p.id),
                };
                return detail;
            },
            async setRouteEnabled(id, modelId, reasoning, on) {
                requireProvider(id);
                const key = formatRouteKey(id, modelId, reasoning);
                parseRouteKey(key);
                const i = data.routesDisabled.indexOf(key);
                if (on && i !== -1)
                    data.routesDisabled.splice(i, 1);
                if (!on && i === -1)
                    data.routesDisabled.push(key);
                emit('config:changed', { section: 'providers' });
            },
            async setAllRoutes(id, on) {
                requireProvider(id);
                for (const m of providerModels(id)) {
                    for (const level of m.levels) {
                        const key = formatRouteKey(id, m.model_id, level.reasoning);
                        const i = data.routesDisabled.indexOf(key);
                        if (on && i !== -1)
                            data.routesDisabled.splice(i, 1);
                        if (!on && i === -1)
                            data.routesDisabled.push(key);
                    }
                }
                emit('config:changed', { section: 'providers' });
            },
            async refreshRoutes() {
                emit('config:changed', { section: 'routes' });
            },
        },
        harnesses: {
            async list() {
                return clone(data.harnesses);
            },
            async setEnabled(slug, enabled) {
                const h = requireHarness(slug);
                h.enabled = enabled;
                emit('config:changed', { section: 'harnesses' });
            },
            async save(h) {
                if (!SLUG_RE.test(h.slug)) {
                    throw new EngineError('validation_failed', `invalid harness slug "${h.slug}"`);
                }
                for (const [, token] of h.command.matchAll(/\{([^}]*)\}/g)) {
                    if (token !== 'model_id' && token !== 'reasoning') {
                        throw new EngineError('validation_failed', `unknown template token "{${token}}" in command`);
                    }
                }
                const existing = data.harnesses.find((x) => x.slug === h.slug);
                if (existing?.builtin) {
                    throw new EngineError('builtin_readonly', `harness "${h.slug}" is built-in and read-only`);
                }
                const saved = { ...clone(h), builtin: false };
                if (existing) {
                    data.harnesses[data.harnesses.indexOf(existing)] = saved;
                }
                else {
                    data.harnesses.push(saved);
                }
                emit('config:changed', { section: 'harnesses' });
            },
            async delete(slug) {
                const h = requireHarness(slug);
                if (h.builtin) {
                    throw new EngineError('builtin_readonly', `harness "${slug}" is built-in and read-only`);
                }
                data.harnesses.splice(data.harnesses.indexOf(h), 1);
                emit('config:changed', { section: 'harnesses' });
            },
            async setProvider(slug, provider, on) {
                const h = requireHarness(slug);
                requireProvider(provider);
                h.providers[provider] = on;
                emit('config:changed', { section: 'harnesses' });
            },
            async setAllProviders(slug, on) {
                const h = requireHarness(slug);
                for (const id of Object.keys(h.providers))
                    h.providers[id] = on;
                emit('config:changed', { section: 'harnesses' });
            },
            async launch(slug, routeKey, profileSlug) {
                const parts = parseRouteKey(routeKey);
                const h = requireHarness(slug);
                const nativeProvider = { claude: 'anthropic', codex: 'openai', copilot: 'github-copilot' }[parts.provider] ?? parts.provider;
                const modelId = h.builtin && (slug === 'opencode' || slug === 'kilo') ? `${nativeProvider}/${parts.modelId}` : parts.modelId;
                let command = h.command
                    .replaceAll('{model_id}', modelId)
                    .replaceAll('{reasoning}', parts.reasoning);
                if (h.builtin && parts.reasoning !== 'default') {
                    if (slug === 'claude' && parts.reasoning !== 'minimal')
                        command += ` --effort ${parts.reasoning}`;
                    if (slug === 'codex')
                        command += ` -c model_reasoning_effort=${parts.reasoning}`;
                }
                if (h.builtin && slug === 'cline')
                    command += ` --provider ${nativeProvider}`;
                recordPickInternal(profileSlug, routeKey);
                return { copied: data.settings.copy_command_instead, command };
            },
        },
        usage: {
            async snapshots(force) {
                const snaps = providersByPriority()
                    .filter((p) => p.on)
                    .map((p) => {
                    const windows = [];
                    const add = (id, label, used) => {
                        if (used !== null) {
                            windows.push({ id, label, used_percent: used, reset_hint: '', unlimited: false });
                        }
                    };
                    add('session', 'Session', p.session);
                    add('weekly', 'Weekly', p.weekly);
                    add('monthly', 'Monthly', p.monthly);
                    return {
                        provider: p.id,
                        plan: p.credits,
                        auth: p.auth,
                        confidence: 'live',
                        stale: false,
                        windows,
                        credits: p.credits,
                        resets: p.resets,
                        failure: '',
                    };
                });
                if (force)
                    emit('usage:updated', {});
                return snaps;
            },
            async setMode(mode) {
                usageMode = mode;
                emit('usage:updated', {});
            },
            async setBackend(backend) {
                usageBackend = backend;
                emit('usage:updated', {});
            },
            async mode() {
                return { mode: usageMode, backend: usageBackend };
            },
        },
        favourites: {
            async list() {
                return data.favourites.map((key) => {
                    const parts = parseRouteKey(key);
                    const model = data.models.find((m) => m.id === parts.modelId);
                    const provider = data.providers.find((p) => p.id === parts.provider);
                    return {
                        route_key: key,
                        model_name: model?.name ?? parts.modelId,
                        route_label: provider
                            ? `${parts.provider} · ${parts.reasoning}`
                            : `no provider · ${parts.reasoning}`,
                        in_range: model !== undefined &&
                            provider !== undefined &&
                            provider.on &&
                            model.providers.includes(parts.provider) &&
                            model.reasoning === parts.reasoning &&
                            !data.routesDisabled.includes(key),
                    };
                });
            },
            async pin(routeKey) {
                parseRouteKey(routeKey);
                if (!data.favourites.includes(routeKey))
                    data.favourites.push(routeKey);
                emit('config:changed', { section: 'favourites' });
            },
            async unpin(routeKey) {
                parseRouteKey(routeKey);
                const i = data.favourites.indexOf(routeKey);
                if (i !== -1)
                    data.favourites.splice(i, 1);
                emit('config:changed', { section: 'favourites' });
            },
        },
        settings: {
            async get() {
                return clone(data.settings);
            },
            async set(s) {
                const next = clone(s);
                const key = next.aa_api_key.trim();
                if (key === '-') {
                    next.aa_api_key_set = false;
                }
                else if (key.length > 0) {
                    next.aa_api_key_set = true;
                }
                else {
                    next.aa_api_key_set = data.settings.aa_api_key_set;
                }
                next.aa_api_key = '';
                if (!next.catalog_repo.trim())
                    next.catalog_repo = 'WD-Mitchell/which-model';
                data.settings = next;
                emit('settings:changed', data.settings);
            },
            async shellSnippets() {
                const profile = data.profiles.find((p) => p.slug === 'balanced_implementation') ?? data.profiles[0];
                let preview = '';
                if (profile) {
                    const top = computeRank(profile, 1).candidates[0];
                    if (top) {
                        preview = `$ wm ${profile.slug}  →  ${top.model_id}  (${top.provider})`;
                    }
                }
                return {
                    alias: 'alias wm="which-model pick"',
                    claude_md: 'Before launching a coding agent, run `wm <profile>` to pick the model.',
                    preview,
                };
            },
        },
        signin: {
            async start(provider) {
                if (provider !== 'claude' &&
                    provider !== 'codex' &&
                    provider !== 'cursor' &&
                    provider !== 'antigravity' &&
                    provider !== 'copilot') {
                    throw new EngineError('validation_failed', `sign-in for ${provider} is not supported`);
                }
                if (activeSignInFlows.has(provider)) {
                    throw new EngineError('conflict', `sign-in already in progress for ${provider}`);
                }
                const flowId = `mock-signin-${++nextSignInFlow}`;
                activeSignInFlows.set(provider, flowId);
                if (provider === 'claude') {
                    return {
                        flow_id: flowId,
                        verification_uri: 'https://claude.ai/oauth/authorize',
                        user_code: '',
                        paste_required: true,
                    };
                }
                if (provider === 'codex') {
                    return {
                        flow_id: flowId,
                        verification_uri: 'https://auth.openai.com/codex/device',
                        user_code: 'WDML-MOCK',
                        paste_required: false,
                    };
                }
                if (provider === 'cursor') {
                    return {
                        flow_id: flowId,
                        verification_uri: 'https://cursor.com/oauth/authorize',
                        user_code: '',
                        paste_required: false,
                    };
                }
                if (provider === 'antigravity') {
                    return {
                        flow_id: flowId,
                        verification_uri: 'https://accounts.google.com/o/oauth2/v2/auth',
                        user_code: '',
                        paste_required: false,
                    };
                }
                return {
                    flow_id: flowId,
                    verification_uri: 'https://github.com/login/device',
                    user_code: 'WDML-MOCK',
                    paste_required: false,
                };
            },
            async confirm(provider, flowId, accountName) {
                if (activeSignInFlows.get(provider) !== flowId) {
                    throw new EngineError('validation_failed', 'sign-in attempt changed');
                }
                if (provider === 'claude') {
                    await new Promise((resolve, reject) => {
                        confirmWaiters.set(flowId, { resolve, reject });
                    });
                }
                if (activeSignInFlows.get(provider) !== flowId) {
                    throw new EngineError('validation_failed', 'sign-in cancelled');
                }
                activeSignInFlows.delete(provider);
                confirmWaiters.delete(flowId);
                const accounts = mockAccounts[provider] ?? [];
                const existing = accounts.findIndex((account) => account.name === accountName);
                const ref = provider === 'cursor' ? 'cursor-agent' : 'which-model';
                if (existing >= 0) {
                    accounts[existing] = { name: accountName, kind: 'oauth', ref };
                }
                else {
                    accounts.push({ name: accountName, kind: 'oauth', ref });
                }
                mockAccounts[provider] = accounts.map((account) => ({ ...account }));
                emit('config:changed', { section: 'providers' });
                emit('usage:updated', {});
            },
            async submitCode(provider, flowId, _code) {
                if (activeSignInFlows.get(provider) !== flowId) {
                    throw new EngineError('validation_failed', 'sign-in attempt changed');
                }
                const waiter = confirmWaiters.get(flowId);
                if (!waiter) {
                    throw new EngineError('validation_failed', 'no pasted-code confirmation is waiting');
                }
                confirmWaiters.delete(flowId);
                waiter.resolve();
            },
            async cancel(provider, flowId) {
                const activeFlowId = activeSignInFlows.get(provider);
                if (activeFlowId === undefined)
                    return;
                if (activeFlowId !== flowId) {
                    throw new EngineError('validation_failed', 'sign-in attempt changed');
                }
                activeSignInFlows.delete(provider);
                const waiter = confirmWaiters.get(flowId);
                if (waiter) {
                    confirmWaiters.delete(flowId);
                    waiter.reject(new EngineError('validation_failed', 'sign-in cancelled'));
                }
            },
            async saveAPIKey(provider, accountName, apiKey) {
                if (!accountName.trim() || !apiKey.trim()) {
                    throw new EngineError('validation_failed', 'account name and API key are required');
                }
                const accounts = mockAccounts[provider] ?? [];
                const existing = accounts.findIndex((account) => account.name === accountName);
                const next = { name: accountName, kind: 'token', ref: 'which-model' };
                if (existing >= 0)
                    accounts[existing] = next;
                else
                    accounts.push(next);
                mockAccounts[provider] = accounts.map((account) => ({ ...account }));
                emit('config:changed', { section: 'providers' });
                emit('usage:updated', {});
            },
        },
        window: {
            async openSettings() { },
            async closeSettings() { },
            async hidePopover() { },
            async quit() { },
            async copyToClipboard(_text) { },
            async openURL(_url) { },
            async setPopoverHeight(_height) { },
            async setTrayPick(_profileName, _modelName, _reasoning, _provider) { },
        },
        on(event, cb) {
            let set = listeners.get(event);
            if (!set) {
                set = new Set();
                listeners.set(event, set);
            }
            set.add(cb);
            return () => {
                set.delete(cb);
            };
        },
    };
    return host;
}
