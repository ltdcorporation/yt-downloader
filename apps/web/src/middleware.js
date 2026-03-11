import { NextResponse } from "next/server";

function unauthorized() {
	const response = new NextResponse("Authentication required", {
		status: 401,
	});
	response.headers.set("WWW-Authenticate", 'Basic realm="admin"');
	return response;
}

function safeEqual(left, right) {
	const leftBytes = new TextEncoder().encode(left);
	const rightBytes = new TextEncoder().encode(right);
	if (leftBytes.length !== rightBytes.length) {
		return false;
	}

	let mismatch = 0;
	for (let index = 0; index < leftBytes.length; index += 1) {
		mismatch |= leftBytes[index] ^ rightBytes[index];
	}
	return mismatch === 0;
}

export function middleware(request) {
	const expectedUser = process.env.ADMIN_BASIC_AUTH_USER || "";
	const expectedPass = process.env.ADMIN_BASIC_AUTH_PASS || "";
	if (!expectedUser || !expectedPass) {
		return unauthorized();
	}

	const authHeader = request.headers.get("authorization") || "";
	if (!authHeader.startsWith("Basic ")) {
		return unauthorized();
	}

	let decoded = "";
	try {
		decoded = atob(authHeader.slice(6));
	} catch {
		return unauthorized();
	}

	const separator = decoded.indexOf(":");
	if (separator < 0) {
		return unauthorized();
	}
	const user = decoded.slice(0, separator);
	const pass = decoded.slice(separator + 1);

	if (!safeEqual(user, expectedUser) || !safeEqual(pass, expectedPass)) {
		return unauthorized();
	}

	return NextResponse.next();
}

export const config = {
	matcher: ["/admin/:path*"],
};
