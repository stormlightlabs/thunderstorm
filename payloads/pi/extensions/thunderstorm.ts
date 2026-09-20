import { spawn } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const denied = ["gh pr merge","gh pr review","git push","git merge"];
const gates = [{"event":"PostToolUse","matcher":"Write|Edit","name":"check-documents.sh","timeout":10}];

function shellWord(word: string): string {
	const escaped = word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	return "[\"']?" + escaped + "[\"']?";
}

function commandPattern(prefix: string): RegExp {
	const words = prefix.split(/\s+/).map(shellWord).join("\\s+");
	return new RegExp("(^|[;&|()\\n]\\s*)" + words + "(?=\\s|$|[;&|()])");
}

// Pi has no hooks, so a write reaches the gate through the same script the
// other two harnesses register, with this handler translating Pi's event into
// the shape that script reads and its reply back into a tool result.
function runGate(root: string, gate: { name: string; timeout: number }, event: object): Promise<string> {
	return new Promise((done) => {
		const child = spawn(resolve(root, "hooks", gate.name), [], { stdio: ["pipe", "pipe", "inherit"] });
		const timer = setTimeout(() => child.kill("SIGKILL"), gate.timeout * 1000);
		let out = "";
		child.stdout.on("data", (chunk) => {
			out += chunk;
		});
		child.on("error", () => {
			clearTimeout(timer);
			done("");
		});
		child.on("close", () => {
			clearTimeout(timer);
			done(out);
		});
		child.stdin.on("error", () => {});
		child.stdin.end(JSON.stringify(event));
	});
}

export default function (pi) {
	const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
	process.env.THUNDERSTORM_PLUGIN_ROOT = root;
	const patterns = denied.map((prefix) => ({ prefix, pattern: commandPattern(prefix) }));
	const writeGates = gates
		.filter((gate) => gate.event === "PostToolUse")
		.map((gate) => ({ ...gate, tools: new RegExp("^(" + gate.matcher + ")$", "i") }));

	pi.on("tool_call", async (event) => {
		if (event.toolName !== "bash") return undefined;
		const command = event.input.command as string;
		const match = patterns.find(({ pattern }) => pattern.test(command));
		if (!match) return undefined;
		return {
			block: true,
			reason: match.prefix + " is reserved for a human in a thunderstorm run.",
		};
	});

	pi.on("tool_result", async (event) => {
		const gate = writeGates.find((candidate) => candidate.tools.test(event.toolName));
		if (!gate) return undefined;
		const reply = await runGate(root, gate, {
			hook_event_name: "PostToolUse",
			tool_name: event.toolName,
			cwd: process.cwd(),
			tool_input: event.input,
		});
		if (!reply.trim()) return undefined;
		let context: string | undefined;
		try {
			context = JSON.parse(reply)?.hookSpecificOutput?.additionalContext;
		} catch {
			return undefined;
		}
		if (!context) return undefined;
		return { content: [...event.content, { type: "text", text: context }] };
	});
}
