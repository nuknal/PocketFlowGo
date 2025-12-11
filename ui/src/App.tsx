import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { Layout } from './components/layout/Layout'
import Dashboard from './pages/Dashboard'
import Flows from './pages/Flows'
import FlowDetails from './pages/FlowDetails'
import Tasks from './pages/Tasks'
import TaskDetails from './pages/TaskDetails'
import Workers from './pages/Workers'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="flows" element={<Flows />} />
          <Route path="flows/:id" element={<FlowDetails />} />
          <Route path="tasks" element={<Tasks />} />
          <Route path="tasks/:id" element={<TaskDetails />} />
          <Route path="workers" element={<Workers />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
