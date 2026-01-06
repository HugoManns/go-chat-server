import {Injectable} from '@angular/core';
import { Observable, Subject } from 'rxjs';

export interface ChatMessage {
    sender?: string;
    content?: string;
}

@Injectable({ providedIn: 'root'})
export class SocketService {
    private socket?: WebSocket;
    private stream$ = new Subject<ChatMessage>();

    connect(url = 'ws://localhost:12345/ws'): Observable<ChatMessage> {
        if(this.socket) return this.stream$.asObservable();

        this.socket = new WebSocket(url);
        this.socket.onopen = () => this.stream$.next({ content: '/Connected'});
        this.socket.onclose = () => this.stream$.next({ content: '/Disconnected'});
        this.socket.onmessage = (event) => this.stream$.next(JSON.parse(event.data));
        this.socket.onerror = () => this.stream$.next({ content: '/Error'});

        return this.stream$.asObservable();
    }

    send(text: string) {
        if(!this.socket || this.socket.readyState !== WebSocket.OPEN) return;
        this.socket.send(text);
    }

    close() {
        this.socket?.close();
    }
}