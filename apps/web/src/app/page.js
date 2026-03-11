"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import { getLocale, t } from "@/lib/i18n";

function extractErrorMessage(payload, fallback) {
	if (!payload || typeof payload !== "object") {
		return fallback;
	}
	if (typeof payload.error === "string" && payload.error.trim()) {
		return payload.error.trim();
	}
	return fallback;
}

function formatDuration(totalSeconds) {
	if (!totalSeconds || Number.isNaN(totalSeconds)) {
		return "-";
	}
	const seconds = Math.max(0, Number(totalSeconds));
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	const s = Math.floor(seconds % 60);
	if (h > 0) {
		return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
	}
	return `${m}:${String(s).padStart(2, "0")}`;
}

export default function Home() {
	const [lang, setLang] = useState("id");
	const [url, setURL] = useState("");
	const [formats, setFormats] = useState([]);
	const [meta, setMeta] = useState(null);
	const [isResolving, setIsResolving] = useState(false);
	const [resolveError, setResolveError] = useState("");
	const [mp3Job, setMp3Job] = useState(null);
	const [isQueueing, setIsQueueing] = useState(false);
	const locale = getLocale(lang);
	const tx = t(locale);
	const apiBase = useMemo(
		() => process.env.NEXT_PUBLIC_API_BASE_URL || "http://127.0.0.1:8080",
		[],
	);

	useEffect(() => {
		if (!mp3Job?.id) {
			return undefined;
		}
		if (mp3Job.status === "done" || mp3Job.status === "failed") {
			return undefined;
		}

		const intervalID = setInterval(async () => {
			try {
				const response = await fetch(`${apiBase}/v1/jobs/${mp3Job.id}`);
				if (!response.ok) {
					return;
				}
				const payload = await response.json();
				setMp3Job(payload);
			} catch {
				// Ignore transient polling failures.
			}
		}, 2000);

		return () => clearInterval(intervalID);
	}, [apiBase, mp3Job?.id, mp3Job?.status]);

	async function onResolve(event) {
		event.preventDefault();
		if (!url.trim()) {
			setResolveError(tx.errURLRequired);
			return;
		}

		setResolveError("");
		setIsResolving(true);
		setFormats([]);
		setMeta(null);
		setMp3Job(null);

		try {
			const response = await fetch(`${apiBase}/v1/youtube/resolve`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ url: url.trim() }),
			});
			const payload = await response.json();
			if (!response.ok) {
				setResolveError(extractErrorMessage(payload, tx.errResolveFailed));
				return;
			}
			setMeta({
				title: payload.title,
				thumbnail: payload.thumbnail,
				durationSeconds: payload.duration_seconds,
			});
			setFormats(Array.isArray(payload.formats) ? payload.formats : []);
		} catch {
			setResolveError(tx.errNetwork);
		} finally {
			setIsResolving(false);
		}
	}

	async function onDownload(item) {
		if (!url.trim()) {
			setResolveError(tx.errURLRequired);
			return;
		}

		if (item.type === "mp4") {
			const params = new URLSearchParams({
				url: url.trim(),
				format_id: item.id,
			});
			window.location.assign(`${apiBase}/v1/download/mp4?${params.toString()}`);
			return;
		}

		setIsQueueing(true);
		setMp3Job({
			id: "",
			status: "queueing",
		});

		try {
			const response = await fetch(`${apiBase}/v1/jobs/mp3`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ url: url.trim() }),
			});
			const payload = await response.json();
			if (!response.ok) {
				setMp3Job({
					status: "failed",
					error: extractErrorMessage(payload, tx.errQueueFailed),
				});
				return;
			}

			setMp3Job({
				id: payload.job_id,
				status: payload.status || "queued",
			});
		} catch {
			setMp3Job({
				status: "failed",
				error: tx.errNetwork,
			});
		} finally {
			setIsQueueing(false);
		}
	}

	return (
		<main>
			<div className="top-row">
				<button type="button" onClick={() => setLang("id")}>
					ID
				</button>
				<button type="button" onClick={() => setLang("en")}>
					EN
				</button>
			</div>

			<section className="card">
				<h1>{tx.appTitle}</h1>
				<p>{tx.subtitle}</p>
				<form onSubmit={onResolve}>
					<label htmlFor="youtube-url">{tx.inputLabel}</label>
					<input
						id="youtube-url"
						value={url}
						onChange={(event) => setURL(event.target.value)}
						placeholder={tx.inputPlaceholder}
					/>
					<div style={{ marginTop: 10 }}>
						<button type="submit" disabled={isResolving}>
							{isResolving ? tx.resolving : tx.stepResolve}
						</button>
					</div>
				</form>
				{resolveError ? <p className="error-text">{resolveError}</p> : null}
			</section>

			{meta ? (
				<section className="card">
						<div className="video-meta">
							{meta.thumbnail ? (
								<Image
									src={meta.thumbnail}
									alt={meta.title || "thumbnail"}
									width={320}
									height={180}
								/>
							) : null}
						<div>
							<h2>{meta.title || "-"}</h2>
							<p>
								{tx.duration}: {formatDuration(meta.durationSeconds)}
							</p>
						</div>
					</div>
				</section>
			) : null}

			<section className="card">
				<h2 style={{ marginTop: 0 }}>{tx.sectionFormats}</h2>
				{formats.length === 0 ? (
					<p>{tx.noFormats}</p>
				) : (
					<table className="list">
						<thead>
							<tr>
								<th>{tx.quality}</th>
								<th>{tx.container}</th>
								<th>{tx.action}</th>
							</tr>
						</thead>
						<tbody>
							{formats.map((item) => (
								<tr key={item.id}>
									<td>{item.quality}</td>
									<td>{item.container}</td>
									<td>
										<button
											type="button"
											onClick={() => onDownload(item)}
											disabled={isQueueing && item.type === "mp3"}
										>
											{item.type === "mp3" && isQueueing ? tx.queueing : tx.stepDownload}
										</button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
				<p className="muted">{tx.note}</p>
			</section>

			{mp3Job ? (
				<section className="card">
					<h2 style={{ marginTop: 0 }}>{tx.mp3StatusTitle}</h2>
					<p>
						{tx.status}: <strong>{mp3Job.status || "-"}</strong>
					</p>
					{mp3Job.id ? (
						<p className="muted">
							{tx.jobID}: {mp3Job.id}
						</p>
					) : null}
					{mp3Job.error ? <p className="error-text">{mp3Job.error}</p> : null}
					{mp3Job.download_url ? (
						<p>
							<a href={mp3Job.download_url} target="_blank" rel="noreferrer">
								{tx.downloadMP3}
							</a>
						</p>
					) : null}
				</section>
			) : null}
		</main>
	);
}
