// Usage: node main.ts [name]. Prints a greeting; --help prints this text.
const usage = "usage: main.ts [name]";
const args: string[] = process.argv.slice(2);

if (args.includes("--help") || args.includes("-h")) {
  console.log(usage);
} else if (args.length > 1) {
  console.error(usage);
  process.exitCode = 2;
} else {
  console.log(`Hello, ${args[0] ?? "world"}!`);
}
