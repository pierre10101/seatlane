import { BrowserRouter, Link, Route, Routes } from 'react-router-dom'
import { ArrowLeft, Compass } from 'lucide-react'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Button } from '@/components/ui/button'
import { SiteHeader } from '@/components/site-header'
import { StateCard } from '@/components/states'
import { EventsPage } from '@/pages/events-page'
import { EventPage } from '@/pages/event-page'
import { ThemeProvider } from '@/hooks/use-theme'
import { AuthProvider } from '@/hooks/use-auth'
import { AuthPage } from '@/pages/auth-page'

export default function App() {
  return (
    <ThemeProvider>
      <TooltipProvider delayDuration={200}>
        <BrowserRouter>
          <AuthProvider>
          <SiteHeader />
          <Routes>
            <Route path="/" element={<EventsPage />} />
            <Route path="/events/:id" element={<EventPage />} />
            <Route path="/sign-in" element={<AuthPage key="sign-in" mode="sign-in" />} />
            <Route path="/sign-up" element={<AuthPage key="sign-up" mode="sign-up" />} />
            <Route
              path="*"
              element={
                <main className="px-4 py-20">
                  <StateCard icon={Compass} title="This page took a wrong turn" action={<Button asChild variant="outline"><Link to="/"><ArrowLeft /> All events</Link></Button>}>
                    The page you are looking for does not exist.
                  </StateCard>
                </main>
              }
            />
          </Routes>
          <footer className="mx-auto max-w-7xl px-4 pb-10 text-xs text-muted-foreground sm:px-6">
            Seatlane · every seat rule runs on the server, reviewed in plain English.
          </footer>
          <Toaster position="top-center" richColors={false} closeButton />
          </AuthProvider>
        </BrowserRouter>
      </TooltipProvider>
    </ThemeProvider>
  )
}
