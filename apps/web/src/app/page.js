"use client";

import { useState } from "react";
import { getLocale, t } from "@/lib/i18n";

const demoFormats = [
  { id: "18", quality: "360p", container: "mp4", type: "mp4" },
  { id: "22", quality: "720p", container: "mp4", type: "mp4" },
  { id: "140", quality: "audio", container: "mp3 128kbps", type: "mp3" }
];

export default function Home() {
  const [lang, setLang] = useState("id");
  const [url, setURL] = useState("");
  const [formats, setFormats] = useState([]);
  const locale = getLocale(lang);
  const tx = t(locale);

  function onResolve(event) {
    event.preventDefault();
    if (!url) return;
    // Placeholder until API resolve endpoint is wired.
    setFormats(demoFormats);
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
            <button type="submit">{tx.stepResolve}</button>
          </div>
        </form>
      </section>

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
                  <td>
                    {item.quality}
                    {item.quality === "720p" ? (
                      <span className="pill">1080p unavailable</span>
                    ) : null}
                  </td>
                  <td>{item.container}</td>
                  <td>
                    <button type="button">{tx.stepDownload}</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <p className="muted">{tx.note}</p>
      </section>
    </main>
  );
}
