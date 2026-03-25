"use client";

import { useDarkMode } from "@/hooks/useDarkMode";
import Navbar from "@/components/shared/Navbar";
import HeroSection from "@/components/shared/HeroSection";
import FeaturesSection from "@/components/shared/FeaturesSection";
import Footer from "@/components/shared/Footer";

export default function Home() {
  const { isDark } = useDarkMode();

  return (
    <div className="relative flex min-h-screen w-full flex-col overflow-x-hidden">
      <div className="layout-container flex h-full grow flex-col">
        <Navbar />
        <HeroSection />
        <FeaturesSection />
        <Footer />
      </div>
    </div>
  );
}
