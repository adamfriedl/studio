/**
 * Thin Cursor cloud helper for Studio (create / resume / send / wait / ping).
 * Stdout: one JSON object. Secrets only via CURSOR_API_KEY env — never argv.
 *
 *   cursor-helper ping
 *   cursor-helper create --repo URL [--ref main] [--model id] [--auto-pr] [--prompt-file PATH|-]
 *   cursor-helper followup --agent-id bc-... [--prompt-file PATH|-]
 */
import { readFileSync } from "node:fs";
import { Agent, Cursor, CursorAgentError } from "@cursor/sdk";

type Out = {
  ok: boolean;
  agentId?: string;
  agentURL?: string;
  runId?: string;
  status?: string;
  prURL?: string;
  branch?: string;
  message?: string;
  error?: string;
};

function apiKey(): string {
  const k = process.env.CURSOR_API_KEY?.trim();
  if (!k) throw new Error("CURSOR_API_KEY required");
  return k;
}

function readPrompt(path: string | undefined): string {
  if (!path || path === "-") {
    return readFileSync(0, "utf8");
  }
  return readFileSync(path, "utf8");
}

function emit(o: Out, code: number): never {
  process.stdout.write(JSON.stringify(o) + "\n");
  process.exit(code);
}

function flag(args: string[], name: string): string | undefined {
  const i = args.indexOf(name);
  if (i === -1) return undefined;
  return args[i + 1];
}

function has(args: string[], name: string): boolean {
  return args.includes(name);
}

async function ping() {
  const key = apiKey();
  await Cursor.me({ apiKey: key });
  emit({ ok: true, message: "cursor ok" }, 0);
}

async function create(args: string[]) {
  const key = apiKey();
  const repo = flag(args, "--repo");
  if (!repo) throw new Error("--repo required");
  const ref = flag(args, "--ref") ?? "main";
  const model = flag(args, "--model") ?? process.env.STUDIO_CURSOR_MODEL ?? "composer-2.5";
  const autoPR = has(args, "--auto-pr");
  const prompt = readPrompt(flag(args, "--prompt-file") ?? "-");

  await using agent = await Agent.create({
    apiKey: key,
    model: { id: model },
    cloud: {
      repos: [{ url: repo, startingRef: ref }],
      autoCreatePR: autoPR,
      skipReviewerRequest: true,
      metadata: { studio: "1" },
    },
  });

  const run = await agent.send(prompt);
  const result = await run.wait();
  const git = (result as { git?: { branches?: Array<{ prUrl?: string; branch?: string }> } }).git;
  const branchInfo = git?.branches?.[0];

  emit(
    {
      ok: result.status !== "error",
      agentId: agent.agentId,
      runId: run.id,
      status: result.status,
      prURL: branchInfo?.prUrl,
      branch: branchInfo?.branch,
      message: typeof result.result === "string" ? result.result : undefined,
    },
    result.status === "error" ? 2 : 0,
  );
}

async function followup(args: string[]) {
  const key = apiKey();
  const agentId = flag(args, "--agent-id");
  if (!agentId) throw new Error("--agent-id required");
  const prompt = readPrompt(flag(args, "--prompt-file") ?? "-");

  await using agent = await Agent.resume(agentId, { apiKey: key });
  const run = await agent.send(prompt);
  const result = await run.wait();
  const git = (result as { git?: { branches?: Array<{ prUrl?: string; branch?: string }> } }).git;
  const branchInfo = git?.branches?.[0];

  emit(
    {
      ok: result.status !== "error",
      agentId: agent.agentId,
      runId: run.id,
      status: result.status,
      prURL: branchInfo?.prUrl,
      branch: branchInfo?.branch,
      message: typeof result.result === "string" ? result.result : undefined,
    },
    result.status === "error" ? 2 : 0,
  );
}

async function main() {
  const [cmd, ...args] = process.argv.slice(2);
  try {
    switch (cmd) {
      case "ping":
        await ping();
        break;
      case "create":
        await create(args);
        break;
      case "followup":
        await followup(args);
        break;
      default:
        throw new Error("usage: cursor-helper ping|create|followup ...");
    }
  } catch (err) {
    if (err instanceof CursorAgentError) {
      emit({ ok: false, error: err.message, message: `retryable=${err.isRetryable}` }, 1);
    }
    const msg = err instanceof Error ? err.message : String(err);
    emit({ ok: false, error: msg }, 1);
  }
}

main();
