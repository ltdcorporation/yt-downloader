const sampleJobs = [
  {
    id: "job_01HYE6B5RWM",
    status: "done",
    input_url: "https://youtu.be/demo123",
    output_kind: "mp3",
    created_at: "2026-03-10T11:20:00Z"
  },
  {
    id: "job_01HYE6B7EHM",
    status: "failed",
    input_url: "https://youtube.com/watch?v=demo456",
    output_kind: "mp3",
    created_at: "2026-03-10T11:24:00Z"
  }
];

export default function AdminPage() {
  return (
    <main>
      <section className="card">
        <h1>Admin (View Only)</h1>
        <p>Recent queue jobs and status snapshots.</p>
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
            {sampleJobs.map((job) => (
              <tr key={job.id}>
                <td>{job.id}</td>
                <td>{job.status}</td>
                <td>{job.output_kind}</td>
                <td>{job.created_at}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </main>
  );
}
