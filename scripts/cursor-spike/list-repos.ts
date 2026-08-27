import { Cursor } from "@cursor/sdk";

const apiKey = process.env.CURSOR_API_KEY?.trim();
if (!apiKey) throw new Error("CURSOR_API_KEY required");

const repos = await Cursor.repositories.list({ apiKey });
console.log("count", repos.length);
for (const r of repos) console.log(JSON.stringify(r));
