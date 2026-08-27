/**
 * Phase 1 spike: Cursor cloud create (+ optional resume) against pad-lab.
 * Usage:
 *   CURSOR_API_KEY=... npx --yes tsx spike.ts
 *   CURSOR_API_KEY=... npx --yes tsx spike.ts --resume bc-...
 */
import { Agent, Cursor, CursorAgentError } from "@cursor/sdk";

const apiKey = process.env.CURSOR_API_KEY?.trim();
if (!apiKey) {
  console.error("CURSOR_API_KEY required");
  process.exit(1);
}

const REPO = "https://github.com/adamfriedl/pad-lab";
const resumeId = process.argv.includes("--resume")
  ? process.argv[process.argv.indexOf("--resume") + 1]
  : undefined;

const prompt = `You are running as a Studio spike on adamfriedl/pad-lab.

Issue: the dashboard linked in the README (https://adamfriedl.net/pad-lab/) is not loading. Find the root cause and fix it.

Constraints:
- Keep the diff small. Do not drive-by refactor.
- Discover and run this repo's tests / build if present.
- Do not edit .github/workflows unless the failure is install/setup caused by this change.
- Open a **draft** pull request when the change is testable. Do not merge.
- PR body must include: Studio: https://github.com/adamfriedl/studio (spike — no issue number yet)
  and a short summary of the root cause and fix.
`;

async function ensureRepoConnected() {
  const repos = await Cursor.repositories.list({ apiKey });
  const hit = repos.find((r) => {
    const url = (r as { url?: string }).url ?? "";
    const name = (r as { name?: string }).name ?? "";
    return url.includes("adamfriedl/pad-lab") || name.includes("pad-lab");
  });
  console.log(
    "connected repos:",
    repos.length,
    repos.slice(0, 10).map((r) => (r as { url?: string; name?: string }).url || (r as { name?: string }).name),
  );
  if (!hit) {
    console.error("FAIL: adamfriedl/pad-lab not in Cursor.repositories.list — connect GitHub in Cursor first");
    process.exit(1);
  }
  console.log("pad-lab: connected");
}

async function main() {
  await ensureRepoConnected();

  if (resumeId) {
    console.log("resuming", resumeId);
    await using agent = await Agent.resume(resumeId, { apiKey });
    console.log("agent.id", agent.agentId);
    const run = await agent.send("Confirm you still have the pad-lab repo context. Reply with one short sentence, then stop. Do not open a PR.");
    console.log("run.id", run.id);
    const result = await run.wait();
    console.log("resume result", result.status, result);
    process.exit(result.status === "error" ? 2 : 0);
  }

  console.log("creating cloud agent…");
  await using agent = await Agent.create({
    apiKey,
    model: { id: "composer-2.5" },
    cloud: {
      repos: [{ url: REPO, startingRef: "main" }],
      autoCreatePR: true,
      skipReviewerRequest: true,
      metadata: { studio: "spike", target: "pad-lab" },
    },
  });
  console.log("agent.id", agent.agentId);

  const run = await agent.send(prompt);
  console.log("run.id", run.id);
  try {
    for await (const event of run.stream()) {
      if (event.type === "assistant") {
        for (const block of event.message.content) {
          if (block.type === "text") process.stdout.write(block.text);
        }
      }
    }
  } catch (err) {
    console.error("stream error (continuing to wait)", err);
  }
  const result = await run.wait();
  console.log("\n--- result ---");
  console.log(JSON.stringify(result, null, 2));
  if (result.status === "error") process.exit(2);
}

main().catch((err) => {
  if (err instanceof CursorAgentError) {
    console.error("startup failed:", err.message, "retryable=", err.isRetryable);
    process.exit(1);
  }
  console.error(err);
  process.exit(1);
});
