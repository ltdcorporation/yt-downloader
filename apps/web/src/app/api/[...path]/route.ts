import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const HOP_BY_HOP_HEADERS = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
  "host",
  "content-length",
]);

type RouteContext = {
  params: {
    path: string[];
  };
};

function trimSlashes(input: string): string {
  return input.replace(/^\/+|\/+$/g, "");
}

function resolveUpstreamURL(request: NextRequest, path: string[]): URL {
  const baseURL = process.env.API_BASE_URL || "http://127.0.0.1:18080";
  const upstream = new URL(baseURL);
  const incomingURL = new URL(request.url);

  const basePath = trimSlashes(upstream.pathname);
  const proxyPath = trimSlashes(path.join("/"));
  const joinedPath = [basePath, proxyPath].filter(Boolean).join("/");

  upstream.pathname = `/${joinedPath}`;
  upstream.search = incomingURL.search;

  return upstream;
}

function copyRequestHeaders(request: NextRequest): Headers {
  const headers = new Headers();

  request.headers.forEach((value, key) => {
    if (!HOP_BY_HOP_HEADERS.has(key.toLowerCase())) {
      headers.set(key, value);
    }
  });

  const host = request.headers.get("host");
  if (host && !headers.has("x-forwarded-host")) {
    headers.set("x-forwarded-host", host);
  }

  return headers;
}

function copyResponseHeaders(source: Headers): Headers {
  const headers = new Headers();

  source.forEach((value, key) => {
    if (!HOP_BY_HOP_HEADERS.has(key.toLowerCase())) {
      headers.set(key, value);
    }
  });

  return headers;
}

async function proxyRequest(request: NextRequest, path: string[]): Promise<NextResponse> {
  let upstreamURL: URL;
  try {
    upstreamURL = resolveUpstreamURL(request, path);
  } catch (error) {
    return NextResponse.json(
      {
        error: "invalid API_BASE_URL configuration",
        details: error instanceof Error ? error.message : "unknown error",
      },
      { status: 500 },
    );
  }

  const method = request.method.toUpperCase();
  const hasBody = method !== "GET" && method !== "HEAD";

  const headers = copyRequestHeaders(request);

  const body = hasBody ? await request.arrayBuffer() : undefined;

  try {
    const upstreamResponse = await fetch(upstreamURL, {
      method,
      headers,
      body,
      cache: "no-store",
      redirect: "manual",
    });

    return new NextResponse(upstreamResponse.body, {
      status: upstreamResponse.status,
      statusText: upstreamResponse.statusText,
      headers: copyResponseHeaders(upstreamResponse.headers),
    });
  } catch (error) {
    return NextResponse.json(
      {
        error: "upstream request failed",
        details: error instanceof Error ? error.message : "unknown error",
      },
      { status: 502 },
    );
  }
}

export async function GET(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}

export async function PUT(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}

export async function PATCH(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}

export async function OPTIONS(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}

export async function HEAD(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context.params.path || []);
}
