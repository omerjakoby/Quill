import { useState } from 'react'
import './css/App.css'
import Bar from './components/UpperBar'
import LoginPage from './pages/login'

function App() {
  const [count, setCount] = useState(0)

  return (
    <body>
        <LoginPage/>   
     </body>  
  )
}

export default App
