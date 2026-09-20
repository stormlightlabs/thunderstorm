import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const denied = ["gh pr merge","gh pr review","git push","git merge"];

function shellWord(word: string): string {
	const escaped = word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	return "[\"']?" + escaped + "[\"']?";
}

function commandPattern(prefix: string): RegExp {
	const words = prefix.split(/\s+/).map(shellWord).join("\\s+");
	return new RegExp("(^|[;&|()\\n]\\s*)" + words + "(?=\\s|$|[;&|()])");
}

export default function (pi) {
	process.env.THUNDERSTORM_PLUGIN_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
	const patterns = denied.map((prefix) => ({ prefix, pattern: commandPattern(prefix) }));

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
}
