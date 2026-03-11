export const dynamic = "force-dynamic";

async function fetchAdminJobs() {
	const apiBase =
		process.env.API_BASE_URL ||
		process.env.NEXT_PUBLIC_API_BASE_URL ||
		"http://127.0.0.1:8080";
	const user = process.env.ADMIN_BASIC_AUTH_USER || "";
	const pass = process.env.ADMIN_BASIC_AUTH_PASS || "";

	try {
		const headers = {};
		if (user && pass) {
			const token = Buffer.from(`${user}:${pass}`).toString("base64");
			headers.Authorization = `Basic ${token}`;
		}

		const response = await fetch(`${apiBase}/admin/jobs?limit=30`, {
			cache: "no-store",
			headers,
		});
		const payload = await response.json();

		if (!response.ok) {
			return {
				items: [],
				error: payload?.error || "Failed to load admin jobs",
			};
		}

		return {
			items: Array.isArray(payload.items) ? payload.items : [],
			error: "",
		};
	} catch {
		return {
			items: [],
			error: "Failed to load admin jobs",
		};
	}
}

export default async function AdminPage() {
	const { items, error } = await fetchAdminJobs();

	return (
		<main>
			<section className="card">
				<h1>Admin (View Only)</h1>
				<p>Recent queue jobs and status snapshots.</p>
				{error ? <p className="error-text">{error}</p> : null}
			</section>

			<section className="card">
				<table className="list">
					<thead>
						<tr>
							<th>Job ID</th>
							<th>Status</th>
							<th>Type</th>
							<th>Created</th>
						</tr>
					</thead>
					<tbody>
						{items.length === 0 ? (
							<tr>
								<td colSpan={4}>No jobs yet.</td>
							</tr>
						) : (
							items.map((job) => (
								<tr key={job.id}>
									<td>{job.id}</td>
									<td>{job.status}</td>
									<td>{job.output_kind || "-"}</td>
									<td>{job.created_at || "-"}</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</section>
		</main>
	);
}
